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

	firebase "firebase.google.com/go"
)

var (
	firebaseConfig = &firebase.Config{
		DatabaseURL:   "https://chore-chart-210002.firebaseio.com",
		ProjectID:     "chore-chart-210002",
		StorageBucket: "chore-chart-210002.appspot.com",
	}
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
	UserID  string
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

	message := r.FormValue("message")

	// Create a new Firebase App
	app, err := firebase.NewApp(ctx, firebaseConfig)
	if err != nil {
		params.Notice = "1. Couldn't authenticate. Try logging in again?"
		params.Message = message // Preserve their message for trying again
		indexTemplate.Execute(w, params)
		return
	}

	// Create a new authenticator for the app
	auth, err := app.Auth(ctx)
	if err != nil {
		params.Notice = "2. Couldn't authenticate. Try logging in again?"
		params.Message = message // Preserve for trying again
		indexTemplate.Execute(w, params)
		return
	}

	// Verify the token passed in by the user is valid
	tok, err := auth.VerifyIDTokenAndCheckRevoked(ctx, r.FormValue("token"))
	if err != nil {
		params.Notice = "3. Couldn't authenticate. Try logging in again? " + err.Error()
		params.Message = message // Preserve for trying again
		indexTemplate.Execute(w, params)
		return
	}

	// Use validated token to get user info
	user, err := auth.GetUser(ctx, tok.UID)
	if err != nil {
		params.Notice = "4. Couldn't authenticate. Try logging in again?"
		params.Message = message // Preserve for trying again
		indexTemplate.Execute(w, params)
		return
	}

	post := Post{
		UserID:  user.UID,
		Author:  user.DisplayName,
		Message: message,
		Posted:  time.Now(),
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
