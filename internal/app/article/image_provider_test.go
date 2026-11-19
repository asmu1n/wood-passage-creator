package articleapp

import (
	"errors"
	"reflect"
	"testing"

	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/response"
	"wood-passage-creator/internal/port"
)

type imageCatalogStub struct {
	providers []port.ImageProviderMetadata
}

func (c imageCatalogStub) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	method = method.Normalize()
	for _, provider := range c.providers {
		if provider.Method.Normalize() == method {
			provider.Method = method
			return provider, true
		}
	}
	return port.ImageProviderMetadata{}, false
}

func (c imageCatalogStub) AvailableProviders(allowedMethods []port.ImageMethod) []port.ImageProviderMetadata {
	out := make([]port.ImageProviderMetadata, 0, len(c.providers))
	for _, provider := range c.providers {
		if port.Allow(allowedMethods, provider.Method) {
			out = append(out, provider)
		}
	}
	return out
}

func TestResolveEnabledImageMethods_DefaultsFollowProviderAccess(t *testing.T) {
	service := &Service{imageCatalog: imageCatalogStub{providers: []port.ImageProviderMetadata{
		{Method: port.MethodPexels, Access: port.ImageAccessFree},
		{Method: port.MethodNanoBanana, Access: port.ImageAccessVIP},
		{Method: port.MethodPicsum, Access: port.ImageAccessInternal},
	}}}

	got, err := service.resolveEnabledImageMethods(nil, moduser.Actor{Role: moduser.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	if want := []port.ImageMethod{port.MethodPexels}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normal user defaults=%v, want %v", got, want)
	}

	got, err = service.resolveEnabledImageMethods(nil, moduser.Actor{Role: moduser.RoleVIP})
	if err != nil {
		t.Fatal(err)
	}
	if want := []port.ImageMethod{port.MethodPexels, port.MethodNanoBanana}; !reflect.DeepEqual(got, want) {
		t.Fatalf("VIP defaults=%v, want %v", got, want)
	}
}

func TestResolveEnabledImageMethods_ValidatesCatalogAndAccess(t *testing.T) {
	service := &Service{imageCatalog: imageCatalogStub{providers: []port.ImageProviderMetadata{
		{Method: port.MethodPexels, Access: port.ImageAccessFree},
		{Method: port.MethodNanoBanana, Access: port.ImageAccessVIP},
		{Method: port.MethodPicsum, Access: port.ImageAccessInternal},
	}}}

	got, err := service.resolveEnabledImageMethods(
		[]port.ImageMethod{" pexels ", port.MethodPexels},
		moduser.Actor{Role: moduser.RoleUser},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := []port.ImageMethod{port.MethodPexels}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized methods=%v, want %v", got, want)
	}

	_, err = service.resolveEnabledImageMethods(
		[]port.ImageMethod{port.MethodNanoBanana},
		moduser.Actor{Role: moduser.RoleUser},
	)
	assertBizCode(t, err, response.Forbidden.Biz)
	_, err = service.resolveEnabledImageMethods(
		[]port.ImageMethod{port.MethodPicsum},
		moduser.Actor{Role: moduser.RoleVIP},
	)
	assertBizCode(t, err, response.ParamsError.Biz)
	_, err = service.resolveEnabledImageMethods(
		[]port.ImageMethod{port.MethodMermaid},
		moduser.Actor{Role: moduser.RoleVIP},
	)
	assertBizCode(t, err, response.ParamsError.Biz)
}

func assertBizCode(t *testing.T, err error, want int) {
	t.Helper()
	var bizErr *response.BizError
	if !errors.As(err, &bizErr) {
		t.Fatalf("error=%v, want BizError code %d", err, want)
	}
	if bizErr.BizCode() != want {
		t.Fatalf("biz code=%d, want %d", bizErr.BizCode(), want)
	}
}
