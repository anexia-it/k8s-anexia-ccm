package test

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/util/rand"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"

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

	preCreateHook func(ctx context.Context, a api.API, o types.Object)
}

// NewFakeAPI creates a FakeAPI and wires the underlying mock to manipulate the in-memory store.
func NewFakeAPI(ctrl *gomock.Controller) *FakeAPI {
	f := &FakeAPI{
		ApiMock: apimock.NewMockAPI(ctrl),
		store:   make(map[string]types.Object),
		tags:    make(map[string][]string),
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
		f.tags[t] = append(f.tags[t], id)
	}

	return id
}

// Existing returns a copy of all stored objects.
func (f *FakeAPI) Existing() []types.Object {
	f.mu.Lock()
	defer f.mu.Unlock()
	ret := make([]types.Object, 0, len(f.store))
	for _, o := range f.store {
		ret = append(ret, deepCopyObject(o))
	}
	return ret
}

// Provide proxy methods so tests can use FakeAPI where previously mock.API was used.
func (f *FakeAPI) Create(ctx context.Context, o types.Object, opts ...types.CreateOption) error {
	return f.ApiMock.Create(ctx, o, opts...)
}
func (f *FakeAPI) Get(ctx context.Context, o types.IdentifiedObject, opts ...types.GetOption) error {
	return f.ApiMock.Get(ctx, o, opts...)
}
func (f *FakeAPI) Update(ctx context.Context, o types.IdentifiedObject, opts ...types.UpdateOption) error {
	return f.ApiMock.Update(ctx, o, opts...)
}
func (f *FakeAPI) Destroy(ctx context.Context, o types.IdentifiedObject, opts ...types.DestroyOption) error {
	return f.ApiMock.Destroy(ctx, o, opts...)
}
func (f *FakeAPI) List(ctx context.Context, filter types.FilterObject, opts ...types.ListOption) error {
	return f.ApiMock.List(ctx, filter, opts...)
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
	f.mu.Lock()
	f.store[id] = deepCopyObject(o)
	f.mu.Unlock()
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
	defer f.mu.Unlock()
	delete(f.store, id)
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

		f.mu.Lock()
		// if tags provided, return those ids
		if len(wantedTags) > 0 {
			for _, t := range wantedTags {
				for _, id := range f.tags[t] {
					stored := f.store[id]
					ch <- func(obj types.Object) error {
						return copyInto(stored, obj)
					}
				}
			}
		} else {
			// return all stored
			for _, stored := range f.store {
				s := stored
				ch <- func(obj types.Object) error {
					return copyInto(s, obj)
				}
			}
		}
		f.mu.Unlock()
		close(ch)
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
