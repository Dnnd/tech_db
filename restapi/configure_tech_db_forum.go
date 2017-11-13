// Code generated by go-swagger; DO NOT EDIT.

package restapi

import (
	"crypto/tls"
	"net/http"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/tylerb/graceful"
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/Dnnd/tech_db/controllers"
	"time"
	"github.com/Dnnd/tech_db/database"
)

// This file is safe to edit. Once it exists it will not be overwritten

//go:generate swagger generate server --target .. --name TechDbForum --spec ../../github.com/bozaro/tech-db-forum/swagger.yml

func configureFlags(api *operations.TechDbForumAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *operations.TechDbForumAPI) http.Handler {
	// configure the api here
	timer := time.NewTimer(time.Minute * 6)
	go func() {
		<-timer.C
		database.DB.Exec("VACUUM")
	}()

	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf
	api.JSONConsumer = runtime.JSONConsumer()

	api.BinConsumer = runtime.ByteStreamConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.ClearHandler = operations.ClearHandlerFunc(controllers.ServiceClear)
	api.ForumCreateHandler = operations.ForumCreateHandlerFunc(controllers.CreateForum)

	api.ForumGetOneHandler = operations.ForumGetOneHandlerFunc(controllers.GetForumDetails)
	api.ForumGetThreadsHandler = operations.ForumGetThreadsHandlerFunc(controllers.GetThreadsByForum)

	api.ForumGetUsersHandler = operations.ForumGetUsersHandlerFunc(controllers.ForumGetUsers)
	api.PostGetOneHandler = operations.PostGetOneHandlerFunc(controllers.PostGetOne)

	api.PostUpdateHandler = operations.PostUpdateHandlerFunc(controllers.PostUpdate)

	api.PostsCreateHandler = operations.PostsCreateHandlerFunc(controllers.PostCreateCopy)
	api.StatusHandler = operations.StatusHandlerFunc(controllers.ServiceStatus)
	api.ThreadCreateHandler = operations.ThreadCreateHandlerFunc(controllers.ThreadCreate)

	api.ThreadGetOneHandler = operations.ThreadGetOneHandlerFunc(controllers.ThreadGetOne)

	api.ThreadGetPostsHandler = operations.ThreadGetPostsHandlerFunc(controllers.ThreadsGetPosts)
	api.ThreadUpdateHandler = operations.ThreadUpdateHandlerFunc(controllers.ThreadUpdateOne)
	api.ThreadVoteHandler = operations.ThreadVoteHandlerFunc(controllers.ThreadVote)

	api.UserCreateHandler = operations.UserCreateHandlerFunc(controllers.CreateUser)
	api.UserGetOneHandler = operations.UserGetOneHandlerFunc(controllers.UserGetOne)
	api.UserUpdateHandler = operations.UserUpdateHandlerFunc(controllers.UpdateUser)

	api.ServerShutdown = func() {

	}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *graceful.Server, scheme, addr string) {

}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {

	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
