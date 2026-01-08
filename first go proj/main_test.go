package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorrectHelloWorldUrl(t *testing.T) {
	req, err := http.NewRequest("GET", "/hello", nil)
	if err != nil {
		t.Fatal(err)
	}

	res := httptest.NewRecorder()

	Handler(res, req)

	//check status code
	if status := res.Code; status != http.StatusOK {
		t.Errorf("handler returned incorrect status code: got %v want %v",
			status, http.StatusOK)
	}

	//check response body
	expected := "hello world!"
	if res.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			res.Body.String(), expected)
	}
}

func TestWrongHelloWorldUrl(t *testing.T) {
	req, err := http.NewRequest("GET", "/abcd", nil)

	if err != nil {
		t.Fatal(err)
	}

	res := httptest.NewRecorder()

	NotFoundHandler(res, req)

	if status := res.Code; status != http.StatusNotFound {
		t.Errorf("handler returned incorrect status code: got %v want %v",
			status, http.StatusNotFound)
	}

	expected := "404 Page not found"
	trimmedRes := strings.TrimSpace(res.Body.String())

	if trimmedRes != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			trimmedRes, expected)
	}
}

func TestWrongHTTPMethods(t *testing.T) {
	methods := []string{"POST", "PUT", "PATCH", "DELETE"}

	for _, method := range methods {
		req, err := http.NewRequest(method, "/hello", nil)
		if err != nil {
			t.Fatal(err)
		}

		res := httptest.NewRecorder()

		MethodNotAllowed(res, req)

		if status := res.Code; status != http.StatusMethodNotAllowed {
			t.Errorf("handler returned incorrect status code: got %v want %v",
				res.Code, http.StatusMethodNotAllowed)
		}
	}
}
