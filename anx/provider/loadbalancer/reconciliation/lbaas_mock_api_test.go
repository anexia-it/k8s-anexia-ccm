package reconciliation

// This file is the mocked generic API client for our LBaaS mock in lbaas_mock_test.go
// Maybe we can generify this and provide with go-anxcloud?
// -- Mara @LittleFox94 Grosch, 2022-03-17

import (
	"context"
	"net/http"
	"reflect"
	"sort"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"

	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (ms *lbaasMockServer) handleAPIRequest(o types.Object, op types.Operation, opts types.Options) error {
	switch o.(type) {
	case *lbaasv1.Frontend,
		*lbaasv1.Bind,
		*lbaasv1.Backend,
		*lbaasv1.Server,
		*lbaasv1.LoadBalancer:
		return ms.handleLBaaSRequest(o, op, opts)
	case *corev1.Resource:
		return ms.handleCoreResourceRequest(o, op, opts)
	}

	return api.ErrTypeNotSupported
}

func (ms *lbaasMockServer) handleLBaaSRequest(o types.Object, op types.Operation, opts types.Options) error {
	var data reflect.Value

	switch o.(type) {
	case *lbaasv1.Frontend:
		data = reflect.ValueOf(&ms.frontends)
	case *lbaasv1.Bind:
		data = reflect.ValueOf(&ms.binds)
	case *lbaasv1.Backend:
		data = reflect.ValueOf(&ms.backends)
	case *lbaasv1.Server:
		data = reflect.ValueOf(&ms.servers)
	case *lbaasv1.LoadBalancer:
		if op != types.OperationGet {
			return api.ErrOperationNotSupported
		}

		return nil
	default:
		Fail("unsupported LBaaS resource")
	}

	identifier, err := api.GetObjectIdentifier(o, op != types.OperationList && op != types.OperationCreate)
	if err != nil {
		return err
	}

	switch op {
	case types.OperationList:
		lo := opts.(*types.ListOptions)
		Expect(lo.ObjectChannel).NotTo(BeNil())

		identifiers := data.MapKeys()

		ch := make(chan types.ObjectRetriever, len(identifiers))
		*lo.ObjectChannel = ch

		for _, id := range identifiers {
			ch <- func(o types.Object) error {
				e := data.MapIndex(id)

				if !lo.FullObjects {
					ne := reflect.New(e.Type()).Elem()
					ne.FieldByName("Identifier").Set(e.FieldByName("Identifier"))
					ne.FieldByName("Name").Set(e.FieldByName("Name"))
					e = ne
				}

				v := reflect.ValueOf(o)

				if v.Type().Elem() != e.Type() {
					return api.ErrTypeNotSupported
				}

				v.Elem().Set(e)
				return nil
			}
		}
		close(ch)

		return nil
	case types.OperationCreate:
		identifier = ms.makeIdentifier()

		ov := reflect.ValueOf(o).Elem()
		identifierField := ov.FieldByName("Identifier")
		Expect(identifierField.Interface()).To(BeZero())

		identifierField.SetString(identifier)

		data.Elem().SetMapIndex(identifierField, ov)
		ms.identifierToResource[identifier] = data

		op = types.OperationGet
	case types.OperationUpdate:
		ov := reflect.ValueOf(o).Elem()
		val := data.Elem().MapIndex(reflect.ValueOf(identifier))
		if !val.IsValid() {
			return api.ErrNotFound
		}

		for i := 0; i < val.NumField(); i++ {
			newFieldValue := ov.Field(i)
			newFieldType := ov.Type().Field(i)

			if newFieldType.Name != "Identifier" && !newFieldValue.IsZero() {
				val.Field(i).Set(newFieldValue)
			}
		}

		data.Elem().SetMapIndex(reflect.ValueOf(identifier), val)
		op = types.OperationGet
	}

	// some cases have to return the object afterwards, unify this here
	if op == types.OperationGet || op == types.OperationDestroy {
		ov := reflect.ValueOf(o).Elem()
		val := data.Elem().MapIndex(reflect.ValueOf(identifier))
		if !val.IsValid() {
			return api.ErrNotFound
		}

		ov.Set(val)
	}

	if op == types.OperationDestroy {
		for tag := range ms.tags {
			ms.untagResource(identifier, tag)
		}

		data.Elem().SetMapIndex(reflect.ValueOf(identifier), reflect.Value{})
		delete(ms.identifierToResource, identifier)
	}

	return nil
}

func (ms *lbaasMockServer) handleCoreResourceRequest(o types.Object, op types.Operation, opts types.Options) error {
	v := reflect.ValueOf(o).Elem()

	var tagName string
	var identifier string

	if op == types.OperationGet {
		r := v.Interface().(corev1.Resource)
		identifier = r.Identifier
	} else if op == types.OperationCreate || op == types.OperationDestroy {
		rwt := v.Interface().(corev1.ResourceWithTag)

		identifier = rwt.Identifier
		tagName = rwt.Tag
	} else if op == types.OperationList {
		r := v.Interface().(corev1.Resource)
		Expect(r.Tags).To(HaveLen(1))
		tagName = r.Tags[0]
	}

	if op == types.OperationGet || op == types.OperationCreate || op == types.OperationDestroy {
		if _, ok := ms.identifierToResource[identifier]; !ok {
			return api.ErrNotFound
		}
	}

	switch op {
	case types.OperationList:
		lo := opts.(*types.ListOptions)

		Expect(lo.ObjectChannel).NotTo(BeNil())

		identifiers, ok := ms.tags[tagName]
		if !ok {
			return api.NewHTTPError(http.StatusUnprocessableEntity, "GET", nil, nil)
		}

		ch := make(chan types.ObjectRetriever, len(identifiers))
		*lo.ObjectChannel = ch

		for _, id := range identifiers {
			id := reflect.ValueOf(id)

			data, ok := ms.identifierToResource[id.Interface().(string)]
			Expect(ok).To(BeTrue())

			val := data.Elem().MapIndex(id)
			Expect(val.IsValid()).To(BeTrue())

			ch <- func(o types.Object) error {
				v := reflect.ValueOf(o)

				if v.Type() != reflect.TypeOf(&corev1.Resource{}) {
					return api.ErrTypeNotSupported
				}

				v.Elem().FieldByName("Identifier").Set(id)
				v.Elem().FieldByName("Name").Set(val.FieldByName("Name"))

				if lo.FullObjects {
					return ms.handleCoreResourceRequest(o, types.OperationGet, &types.GetOptions{})
				}
				return nil
			}
		}
		close(ch)

		return nil
	case types.OperationGet:
		valDB, ok := ms.identifierToResource[identifier]
		if !ok {
			return api.ErrNotFound
		}

		val := valDB.Elem().MapIndex(reflect.ValueOf(identifier))
		if !val.IsValid() {
			return api.ErrNotFound
		}

		tags := make([]string, 0)
		for tag, resources := range ms.tags {
			sort.Strings(resources)
			if idx := sort.SearchStrings(resources, identifier); idx < len(resources) && resources[idx] == identifier {
				tags = append(tags, tag)
			}
		}

		r := corev1.Resource{
			Identifier: identifier,
			Name:       val.FieldByName("Name").Interface().(string),
			Tags:       tags,
		}

		switch val.Interface().(type) {
		case lbaasv1.Frontend:
			r.Type.Name = "Frontends"
			r.Type.Identifier = frontendResourceTypeIdentifier
		case lbaasv1.Bind:
			r.Type.Name = "Frontend Binds"
			r.Type.Identifier = bindResourceTypeIdentifier
		case lbaasv1.Backend:
			r.Type.Name = "Backends"
			r.Type.Identifier = backendResourceTypeIdentifier
		case lbaasv1.Server:
			r.Type.Name = "Backend Servers"
			r.Type.Identifier = serverResourceTypeIdentifier
		}

		v.Set(reflect.ValueOf(r))
		return nil
	case types.OperationCreate:
		if !ms.tagResource(identifier, tagName) {
			return api.NewHTTPError(http.StatusUnprocessableEntity, "GET", nil, nil)
		}

		return nil
	case types.OperationDestroy:
		if !ms.untagResource(identifier, tagName) {
			return api.ErrNotFound
		}

		return nil
	default:
		return api.ErrOperationNotSupported
	}
}

func (ms *lbaasMockServer) Create(_ context.Context, o types.Object, opts ...types.CreateOption) error {
	copts := types.CreateOptions{}
	for _, opt := range opts {
		opt.ApplyToCreate(&copts)
	}

	return ms.handleAPIRequest(o, types.OperationCreate, &copts)
}

func (ms *lbaasMockServer) Destroy(_ context.Context, o types.IdentifiedObject, opts ...types.DestroyOption) error {
	dopts := types.DestroyOptions{}
	for _, opt := range opts {
		opt.ApplyToDestroy(&dopts)
	}

	return ms.handleAPIRequest(o, types.OperationDestroy, &dopts)
}

func (ms *lbaasMockServer) Get(_ context.Context, o types.IdentifiedObject, opts ...types.GetOption) error {
	gopts := types.GetOptions{}
	for _, opt := range opts {
		opt.ApplyToGet(&gopts)
	}

	return ms.handleAPIRequest(o, types.OperationGet, &gopts)
}

func (ms *lbaasMockServer) List(_ context.Context, o types.FilterObject, opts ...types.ListOption) error {
	lopts := types.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(&lopts)
	}

	return ms.handleAPIRequest(o, types.OperationList, &lopts)
}

func (ms *lbaasMockServer) Update(_ context.Context, o types.IdentifiedObject, opts ...types.UpdateOption) error {
	uopts := types.UpdateOptions{}
	for _, opt := range opts {
		opt.ApplyToUpdate(&uopts)
	}

	return ms.handleAPIRequest(o, types.OperationUpdate, &uopts)
}
