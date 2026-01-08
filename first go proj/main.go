package main

import (
	"fmt"
	"net/http" // contains methods to implement HTTP clients and servers
	"github.com/gorilla/mux"
)

func main() {
	//declaring a new router
	r := mux.NewRouter()

	//a simple API endpoint, where API receives requests about specific resource (hello world page in this case)
	r.HandleFunc("/hello", Handler).Methods("GET")

	//handles request to non-existent endpoints with custom 404 page not found error
	r.HandleFunc("/", NotFoundHandler).Methods("GET")

	http.ListenAndServe(":8080", r)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello world!")
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "404 Page not found", http.StatusNotFound)
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request){
	http.Error(w, r.Method+" method is not allowed", http.StatusMethodNotAllowed)
}
