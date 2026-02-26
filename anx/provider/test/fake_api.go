package test

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/util/rand"

	anxmock "go.anx.io/go-anxcloud/pkg/api/mock"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/apimock"
)

// FakeAPI provides a lightweight in-memory fake around the generated GoMock API mock.
// It exposes convenience helpers used throughout the old tests (FakeExisting, Existing, SetPreCreateHook)
// while wiring the underlying gomock mock to operate against the in-memory state.
type FakeAPI struct {
	ApiMock *apimock.MockAPI

	mu    sync.Mutex
	store map[string]types.Object
	// tag -> identifiers
	tags map[string][]string
	// type identifier mapping for stored objects
	typeMap map[string]string

	preCreateHook func(ctx context.Context, a api.API, o types.Object)
}

// NewFakeAPI creates a FakeAPI and wires the underlying mock to manipulate the in-memory store.
func NewFakeAPI(ctrl *gomock.Controller) *FakeAPI {
	f := &FakeAPI{
		ApiMock: apimock.NewMockAPI(ctrl),
		store:   make(map[string]types.Object),
		tags:    make(map[string][]string),
		typeMap: make(map[string]string),
	}

	// Wire mock methods to the helper implementations
	f.ApiMock.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(f.doCreate)
	f.ApiMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(f.doGet)
	f.ApiMock.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(f.doList)
	f.ApiMock.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(f.doUpdate)
	f.ApiMock.EXPECT().Destroy(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(f.doDestroy)

	return f
}

// SetPreCreateHook sets a hook that will be called by Create prior to storing the object.
func (f *FakeAPI) SetPreCreateHook(h func(ctx context.Context, a api.API, o types.Object)) {
	f.preCreateHook = h
}

// FakeExisting stores an object in the fake store and optionally tags it. Returns the object's identifier.
func (f *FakeAPI) FakeExisting(o types.Object, tag ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Ensure identifier
	id, _ := types.GetObjectIdentifier(o, false)
	if id == "" {
		id = rand.String(16)
		// try to set Identifier field if present
		v := reflect.ValueOf(o)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.IsValid() {
			f := v.FieldByName("Identifier")
			if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
				f.SetString(id)
			}
		}
	}

	// store a deep copy
	f.store[id] = deepCopyObject(o)

	for _, t := range tag {
		if id == "" {
			continue
		}
		// avoid duplicate entries for the same tag
		ids := f.tags[t]
		found := false
		for _, existing := range ids {
			if existing == id {
				found = true
				break
			}
		}
		if !found {
			f.tags[t] = append(f.tags[t], id)
		}
	}

	// map type to resource type identifier constants used by reconciliation
	switch o := o.(type) {
	case *lbaasv1.Frontend:
		f.typeMap[id] = "da9d14b9d95840c08213de67f9cee6e2"
	case *lbaasv1.Backend:
		f.typeMap[id] = "33164a3066a04a52be43c607f0c5dd8c"
	case *lbaasv1.Bind:
		f.typeMap[id] = "bd24def982aa478fb3352cb5f49aab47"
	case *lbaasv1.Server:
		f.typeMap[id] = "01f321a4875446409d7d8469503a905f"
	default:
		_ = o
	}

	return id
}

// Existing returns a copy of all stored objects.
// Existing returns a slice of *mock.APIObject wrappers for compatibility with
// the original matcher expectations (the matcher expects *mock.APIObject).
// We construct a temporary real mock.API, populate it with our stored objects
// via its FakeExisting helper and return the pointers returned by Inspect.
func (f *FakeAPI) Existing() []*anxmock.APIObject {
	f.mu.Lock()
	defer f.mu.Unlock()

	// create a fresh mock API implementation to produce *anxmock.APIObject pointers
	m := anxmock.NewMockAPI()

	ret := make([]*anxmock.APIObject, 0, len(f.store))
	for _, o := range f.store {
		// use a deep copy to avoid sharing memory
		copyObj := deepCopyObject(o)
		id := m.FakeExisting(copyObj)
		if ao := m.Inspect(id); ao != nil {
			ret = append(ret, ao)
		}
	}
	return ret
}

// Provide proxy methods so tests can use FakeAPI where previously mock.API was used.
func (f *FakeAPI) Create(ctx context.Context, o types.Object, opts ...types.CreateOption) error {
	return f.doCreate(ctx, o, opts...)
}
func (f *FakeAPI) Get(ctx context.Context, o types.IdentifiedObject, opts ...types.GetOption) error {
	return f.doGet(ctx, o, opts...)
}
func (f *FakeAPI) Update(ctx context.Context, o types.IdentifiedObject, opts ...types.UpdateOption) error {
	return f.doUpdate(ctx, o, opts...)
}
func (f *FakeAPI) Destroy(ctx context.Context, o types.IdentifiedObject, opts ...types.DestroyOption) error {
	return f.doDestroy(ctx, o, opts...)
}
func (f *FakeAPI) List(ctx context.Context, filter types.FilterObject, opts ...types.ListOption) error {
	return f.doList(ctx, filter, opts...)
}

// internal implementations wired to gomock
func (f *FakeAPI) doCreate(ctx context.Context, o types.Object, opts ...types.CreateOption) error {
	if f.preCreateHook != nil {
		f.preCreateHook(ctx, f.ApiMock, o)
	}
	// ensure identifier
	id, _ := types.GetObjectIdentifier(o, false)
	if id == "" {
		id = rand.String(16)
		// set if possible
		v := reflect.ValueOf(o)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.IsValid() {
			f := v.FieldByName("Identifier")
			if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
				f.SetString(id)
			}
		}
	}
	// If this is a ResourceWithTag, do not overwrite the target object's store entry.
	// Instead add the identifier to the tag mapping (only if valid and present in store).
	switch rt := o.(type) {
	case *corev1.ResourceWithTag:
		// rt.Identifier contains the target object's identifier
		if rt.Identifier == "" {
			return fmt.Errorf("empty identifier on ResourceWithTag")
		}
		f.mu.Lock()
		// only append if the target exists in store
		if _, ok := f.store[rt.Identifier]; ok {
			ids := f.tags[rt.Tag]
			found := false
			for _, existing := range ids {
				if existing == rt.Identifier {
					found = true
					break
				}
			}
			if !found {
				f.tags[rt.Tag] = append(f.tags[rt.Tag], rt.Identifier)
			}
		}
		f.mu.Unlock()
		return nil
	default:
		f.mu.Lock()
		f.store[id] = deepCopyObject(o)
		// ensure type mapping for lbaas resources created during tests
		switch o := o.(type) {
		case *lbaasv1.Frontend:
			f.typeMap[id] = "da9d14b9d95840c08213de67f9cee6e2"
		case *lbaasv1.Backend:
			f.typeMap[id] = "33164a3066a04a52be43c607f0c5dd8c"
		case *lbaasv1.Bind:
			f.typeMap[id] = "bd24def982aa478fb3352cb5f49aab47"
		case *lbaasv1.Server:
			f.typeMap[id] = "01f321a4875446409d7d8469503a905f"
		default:
			_ = o
		}
		f.mu.Unlock()
	}
	return nil
}

func (f *FakeAPI) doGet(ctx context.Context, o types.IdentifiedObject, opts ...types.GetOption) error {
	id, err := types.GetObjectIdentifier(o, true)
	if err != nil {
		return err
	}
	f.mu.Lock()
	stored, ok := f.store[id]
	f.mu.Unlock()
	if !ok {
		// debug output to help locate missing identifiers during test migration
		keys := make([]string, 0, len(f.store))
		f.mu.Lock()
		for k := range f.store {
			keys = append(keys, k)
		}
		f.mu.Unlock()
		fmt.Printf("FakeAPI.doGet: id %s not found; store keys=%v\n", id, keys)
		return fmt.Errorf("not found")
	}
	return copyInto(stored, o)
}

func (f *FakeAPI) doUpdate(ctx context.Context, o types.IdentifiedObject, opts ...types.UpdateOption) error {
	id, err := types.GetObjectIdentifier(o, true)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[id] = deepCopyObject(o)
	return nil
}

func (f *FakeAPI) doDestroy(ctx context.Context, o types.IdentifiedObject, opts ...types.DestroyOption) error {
	id, err := types.GetObjectIdentifier(o, true)
	if err != nil {
		return err
	}
	f.mu.Lock()
	// remove from store
	delete(f.store, id)
	// also remove id from any tag lists
	for tag, ids := range f.tags {
		newIds := make([]string, 0, len(ids))
		for _, tid := range ids {
			if tid != id {
				newIds = append(newIds, tid)
			}
		}
		if len(newIds) == 0 {
			delete(f.tags, tag)
		} else {
			f.tags[tag] = newIds
		}
	}
	f.mu.Unlock()
	return nil
}

func (f *FakeAPI) doList(ctx context.Context, filter types.FilterObject, opts ...types.ListOption) error {
	lo := &types.ListOptions{}
	for _, o := range opts {
		_ = o.ApplyToList(lo)
	}

	// build list of identifiers to return

	// if filter is a resource with tags, return ids matching the tag
	// we intentionally handle the common pattern used in provider tests
	if lo.ObjectChannel != nil {
		// detect tags via reflection on filter (avoid importing heavy packages here)
		// Try to find a field named "Tags" and if present, match accordingly.
		filterVal := reflect.ValueOf(filter)
		var wantedTags []string
		if filterVal.Kind() == reflect.Ptr {
			filterVal = filterVal.Elem()
		}
		if filterVal.IsValid() {
			tagsField := filterVal.FieldByName("Tags")
			if tagsField.IsValid() && tagsField.Kind() == reflect.Slice {
				for i := 0; i < tagsField.Len(); i++ {
					if tagsField.Index(i).Kind() == reflect.String {
						wantedTags = append(wantedTags, tagsField.Index(i).String())
					}
				}
			}
		}

		ch := make(chan types.ObjectRetriever, 4)
		*lo.ObjectChannel = types.ObjectChannel(ch)

		// snapshot ids and stored objects under lock, then send asynchronously
		f.mu.Lock()
		ids := make([]string, 0)
		storedMap := make(map[string]types.Object)
		if len(wantedTags) > 0 {
			for _, t := range wantedTags {
				for _, id := range f.tags[t] {
					if s, ok := f.store[id]; ok {
						ids = append(ids, id)
						storedMap[id] = s
					}
				}
			}
		} else {
			for id, stored := range f.store {
				ids = append(ids, id)
				storedMap[id] = stored
			}
		}
		f.mu.Unlock()

		// debug: log how many ids we'll return for this List call
		if len(ids) > 8 {
			fmt.Printf("FakeAPI.doList: returning %d ids for tags=%v (showing up to 8)\n", len(ids), wantedTags)
		}

		go func(ids []string, storedMap map[string]types.Object) {
			defer close(ch)
			for _, id := range ids {
				stored := storedMap[id]
				// return a retriever that sets at least the Identifier on the provided object
				ch <- func(id string, stored types.Object) types.ObjectRetriever {
					return func(obj types.Object) error {
						// try to set Identifier field if present
						rv := reflect.ValueOf(obj)
						if rv.Kind() == reflect.Ptr {
							rv = rv.Elem()
						}
						if rv.IsValid() {
							field := rv.FieldByName("Identifier")
							if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
								field.SetString(id)
								// also set Resource.Type.Identifier if present and known
								if typeID, ok := f.typeMap[id]; ok {
									typeField := rv.FieldByName("Type")
									if typeField.IsValid() {
										// Type may be a struct with Identifier field
										ident := typeField.FieldByName("Identifier")
										if ident.IsValid() && ident.CanSet() && ident.Kind() == reflect.String {
											ident.SetString(typeID)
										}
									}
								}
								return nil
							}
						}
						// fallback: copy whole object if types match
						if reflect.TypeOf(obj) == reflect.TypeOf(stored) {
							return copyInto(stored, obj)
						}
						return nil
					}
				}(id, stored)
			}
		}(ids, storedMap)

		return nil
	}

	return nil
}

// deepCopyObject returns a deep copy of the given object using json marshal/unmarshal
func deepCopyObject(o types.Object) types.Object {
	t := reflect.TypeOf(o)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	newv := reflect.New(t).Elem()
	// set fields from source
	srcv := reflect.ValueOf(o)
	if srcv.Kind() == reflect.Ptr {
		srcv = srcv.Elem()
	}
	newv.Set(srcv)
	return newv.Addr().Interface().(types.Object)
}

func copyInto(src types.Object, dst types.Object) error {
	srcv := reflect.ValueOf(src)
	dstv := reflect.ValueOf(dst)
	if srcv.Kind() == reflect.Ptr {
		srcv = srcv.Elem()
	}
	if dstv.Kind() == reflect.Ptr {
		dstv = dstv.Elem()
	}
	if !srcv.IsValid() || !dstv.IsValid() {
		return fmt.Errorf("invalid values for copy")
	}
	if srcv.Type() != dstv.Type() {
		return fmt.Errorf("type mismatch copying object: %v -> %v", srcv.Type(), dstv.Type())
	}
	dstv.Set(srcv)
	return nil
}
