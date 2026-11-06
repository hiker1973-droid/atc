package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarshalRecoveryCase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tower":{"recoveryCase":3,"caseOverride":0}}`))
	}))
	defer srv.Close()
	if n := marshalRecoveryCase(strings.TrimPrefix(srv.URL, "http://")); n != 3 {
		t.Errorf("case = %d, want 3", n)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tower":{"recoveryCase":0}}`))
	}))
	defer bad.Close()
	if n := marshalRecoveryCase(strings.TrimPrefix(bad.URL, "http://")); n != 0 {
		t.Errorf("an unset case = %d, want 0", n)
	}
	if n := marshalRecoveryCase(""); n != 0 {
		t.Errorf("no address = %d, want 0", n)
	}
}
