package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"html/template"
	"net/http"
	"time"
)

var (
	indexTemplate = template.Must(template.ParseFiles("index.html"))
)

type templateParams struct {
	Notice  string
	Name    string
	Message string
	Posts   []Post
}

type Post struct {
	Author  string
	Message string
	Posted  time.Time
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", getIndexHandler).Methods("GET")
	router.HandleFunc("/", postIndexHandler).Methods("POST")
	router.HandleFunc("/delete", deleteIndexHandler).Methods("POST")
	http.Handle("/", router)
	appengine.Main()
}

func getIndexHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure we're at the right url, only allow /
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ctx := appengine.NewContext(r)

	params := templateParams{}

	q := datastore.NewQuery("Post").Order("-Posted").Limit(20)

	if _, err := q.GetAll(ctx, &params.Posts); err != nil {
		log.Errorf(ctx, "Getting posts: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		params.Notice = "Couldn't get latest posts. Refresh?"
		indexTemplate.Execute(w, params)
		return
	}

	indexTemplate.Execute(w, params)

}

func deleteIndexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	post := Post{
		Author:  r.FormValue("name"),
		Message: r.FormValue("message"),
		Posted:  time.Now(),
	}

	q := datastore.NewQuery("Post").Filter("Author =", post.Author).Filter("Message =", post.Message)

	posts := []Post{}
	keys, err := q.GetAll(ctx, &posts)
	if err != nil {
		log.Errorf(ctx, "Getting posts: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	key := keys[0]
	err = datastore.Delete(ctx, key)

	time.Sleep(100 * time.Millisecond)
	http.Redirect(w, r, "/", http.StatusSeeOther)

	// TODO Have name preserved
	// TODO Have notice say message: Post successfully deleted!

}

func postIndexHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure we're at the right url, only allow /
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ctx := appengine.NewContext(r)

	params := templateParams{}

	q := datastore.NewQuery("Post").Order("-Posted").Limit(20)

	if _, err := q.GetAll(ctx, &params.Posts); err != nil {
		log.Errorf(ctx, "Getting posts: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		params.Notice = "Couldn't get latest posts. Refresh?"
		indexTemplate.Execute(w, params)
		return
	}

	// It's a POST request, so handle the form submission.
	post := Post{
		Author:  r.FormValue("name"),
		Message: r.FormValue("message"),
		Posted:  time.Now(),
	}

	if post.Author == "" {
		post.Author = "Anonymous Gopher"
	}
	params.Name = post.Author

	if post.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		params.Notice = "No message provided"
		indexTemplate.Execute(w, params)
		return
	}

	key := datastore.NewIncompleteKey(ctx, "Post", nil)

	if _, err := datastore.Put(ctx, key, &post); err != nil {
		log.Errorf(ctx, "datastore.Put: %v", err)

		w.WriteHeader(http.StatusInternalServerError)
		params.Notice = "Couldn't add new post. Try again?"
		params.Message = post.Message // Preserve their message so they can try again.
		indexTemplate.Execute(w, params)
		return
	}

	// Prepend the post that was just added.
	params.Posts = append([]Post{post}, params.Posts...)

	params.Notice = fmt.Sprintf("Thank you for your submission, %s!", post.Author)
	indexTemplate.Execute(w, params)

}
