package reconciliation

import (
	"reflect"
	"sort"
	"sync"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	"k8s.io/apimachinery/pkg/util/rand"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type LBaaSMock interface {
	FakeExisting(o types.Object, tags ...string) string

	Frontends() []lbaasv1.Frontend
	Binds() []lbaasv1.Bind
	Backends() []lbaasv1.Backend
	Servers() []lbaasv1.Server
}

type lbaasMockServer struct {
	frontends map[string]lbaasv1.Frontend
	binds     map[string]lbaasv1.Bind
	backends  map[string]lbaasv1.Backend
	servers   map[string]lbaasv1.Server

	identifierToResource map[string]reflect.Value
	tags                 map[string][]string

	mutex sync.Mutex
}

func lbaasMockAPI() (api.API, LBaaSMock) {
	ms := lbaasMockServer{
		frontends: make(map[string]lbaasv1.Frontend),
		binds:     make(map[string]lbaasv1.Bind),
		backends:  make(map[string]lbaasv1.Backend),
		servers:   make(map[string]lbaasv1.Server),

		identifierToResource: make(map[string]reflect.Value),
		tags:                 make(map[string][]string),

		mutex: sync.Mutex{},
	}

	return &ms, &ms
}

func (ms *lbaasMockServer) tagResource(identifier, tag string) bool {
	if res, ok := ms.tags[tag]; !ok {
		ms.tags[tag] = make([]string, 0, 1)
	} else {
		sort.Strings(res)

		if idx := sort.SearchStrings(res, identifier); idx < len(res) && res[idx] == identifier {
			return false
		}
	}

	ms.tags[tag] = append(ms.tags[tag], identifier)
	return true
}

func (ms *lbaasMockServer) untagResource(identifier, tag string) bool {
	changed := false

	if tagResources, ok := ms.tags[tag]; ok {
		newResources := make([]string, 0, len(tagResources))
		for _, resource := range tagResources {
			if resource != identifier {
				newResources = append(newResources, resource)
			} else {
				changed = true
			}
		}

		ms.tags[tag] = newResources
	}

	return changed
}

func (ms *lbaasMockServer) listData(m reflect.Value) interface{} {
	ret := reflect.MakeSlice(reflect.SliceOf(m.Type().Elem()), 0, len(m.MapKeys()))

	iter := m.MapRange()
	for iter.Next() {
		ret = reflect.Append(ret, iter.Value())
	}

	return ret.Interface()
}

func (ms *lbaasMockServer) Frontends() []lbaasv1.Frontend {
	return ms.listData(reflect.ValueOf(ms.frontends)).([]lbaasv1.Frontend)
}

func (ms *lbaasMockServer) Binds() []lbaasv1.Bind {
	return ms.listData(reflect.ValueOf(ms.binds)).([]lbaasv1.Bind)
}

func (ms *lbaasMockServer) Backends() []lbaasv1.Backend {
	return ms.listData(reflect.ValueOf(ms.backends)).([]lbaasv1.Backend)
}

func (ms *lbaasMockServer) Servers() []lbaasv1.Server {
	return ms.listData(reflect.ValueOf(ms.servers)).([]lbaasv1.Server)
}

func (ms *lbaasMockServer) FakeExisting(o types.Object, tags ...string) string {
	val := reflect.Indirect(reflect.ValueOf(o))

	identifier := ms.makeIdentifier()
	var data reflect.Value

	switch val.Type() {
	case reflect.TypeOf(lbaasv1.Frontend{}):
		data = reflect.ValueOf(&ms.frontends)
	case reflect.TypeOf(lbaasv1.Bind{}):
		data = reflect.ValueOf(&ms.binds)
	case reflect.TypeOf(lbaasv1.Backend{}):
		data = reflect.ValueOf(&ms.backends)
	case reflect.TypeOf(lbaasv1.Server{}):
		data = reflect.ValueOf(&ms.servers)
	default:
		Fail("invalid object type for FakeExisting")
	}

	val.FieldByName("Identifier").SetString(identifier)
	data.Elem().SetMapIndex(reflect.ValueOf(identifier), val)

	ms.identifierToResource[identifier] = data

	for _, tag := range tags {
		ms.tagResource(identifier, tag)
	}

	return identifier
}

func (ms *lbaasMockServer) makeIdentifier() string {
	return rand.String(32)
}

var _ = Describe("mock server", func() {
	var mock LBaaSMock

	BeforeEach(func() {
		_, mock = lbaasMockAPI()
	})

	Context("FakeExisting method", func() {
		It("stores a backend", func() {
			identifier := mock.FakeExisting(&lbaasv1.Backend{
				Name: "test",
				Mode: lbaasv1.TCP,
			})

			backends := mock.Backends()
			Expect(backends).To(HaveLen(1))
			Expect(backends[0].Identifier).To(Equal(identifier))
			Expect(backends[0].Name).To(Equal("test"))
			Expect(backends[0].Mode).To(Equal(lbaasv1.TCP))
		})
	})
})
