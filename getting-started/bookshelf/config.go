// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bookshelf

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"gopkg.in/mgo.v2"

	"github.com/gorilla/sessions"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	DB          BookDatabase
	OAuthConfig *oauth2.Config

	StorageBucket     *storage.BucketHandle
	StorageBucketName string
	StorageBucketName1 string

	SessionStore sessions.Store

	PubsubClient *pubsub.Client

	// Force import of mgo library.
	_ mgo.Session
)

const PubsubTopicID = "fill-book-details"

func init() {
	fmt.Printf("\nconfigureCloudSQL!\n")
	var err error

	// To use the in-memory test database, uncomment the next line.
	//DB = newMemoryDB()

	// [START cloudsql]
	// To use Cloud SQL, uncomment the following lines, and update the username,
	// password and instance connection string. When running locally,
	// localhost:3306 is used, and the instance name is ignored.
	DB, err = configureCloudSQL(cloudSQLConfig{
		Username: "root",
		Password: "123",
		// The connection name of the Cloud SQL v2 instance, i.e.,
		// "project:region:instance-id"
		// Cloud SQL v1 instances are not supported.
		Instance: "ruihuang-bookshelf01:us-east4:library",
	})
	// [END cloudsql]

	// [START mongo]
	// To use Mongo, uncomment the next lines and update the address string and
	// optionally, the credentials.
	//
	// var cred *mgo.Credential
	// DB, err = newMongoDB("localhost", cred)
	// [END mongo]

	// [START datastore]
	// To use Cloud Datastore, uncomment the following lines and update the
	// project ID.
	// More options can be set, see the google package docs for details:
	// http://godoc.org/golang.org/x/oauth2/google
	//
	// DB, err = configureDatastoreDB("<your-project-id>")
	// [END datastore]

	if err != nil {
		log.Fatal(err)
	}

	// [START storage]
	// To configure Cloud Storage, uncomment the following lines and update the
	// bucket name.
	//
	StorageBucketName = "ruihuang-bookshelf-bucket01"
	StorageBucket, err = configureStorage(StorageBucketName)
	// [END storage]

	// [START auth]
	// To enable user sign-in, uncomment the following lines and update the
	// Client ID and Client Secret.
	// You will also need to update OAUTH2_CALLBACK in app.yaml when pushing to
	// production.
	//
	OAuthConfig = configureOAuthClient("64013836047-c3nk16plvan7ll1erhu8ngsplsblm7r1.apps.googleusercontent.com", "DWJyCffv2mYZLF0cBJOvvinA")
	// [END auth]

	// [START sessions]
	// Configure storage method for session-wide information.
	// Update "something-very-secret" with a hard to guess string or byte sequence.
	cookieStore := sessions.NewCookieStore([]byte("something-very-secret"))
	cookieStore.Options = &sessions.Options{
		HttpOnly: true,
	}
	SessionStore = cookieStore
	// [END sessions]

	// [START pubsub]
	// To configure Pub/Sub, uncomment the following lines and update the project ID.
	//
	PubsubClient, err = configurePubsub("ruihuang-bookshelf01")
	// [END pubsub]

	if err != nil {
		log.Fatal(err)
	}
}

func configureDatastoreDB(projectID string) (BookDatabase, error) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return newDatastoreDB(client)
}

func configureStorage(bucketID string) (*storage.BucketHandle, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile("/usr/local/google/home/ruihuang/Downloads/ruihuang-bookshelf01-4a4ea6668b1f.json"))
	if err != nil {
		return nil, err
	}
	return client.Bucket(bucketID), nil
}

func configurePubsub(projectID string) (*pubsub.Client, error) {
	if _, ok := DB.(*memoryDB); ok {
		return nil, errors.New("Pub/Sub worker doesn't work with the in-memory DB " +
			"(worker does not share its memory as the main app). Configure another " +
			"database in bookshelf/config.go first (e.g. MySQL, Cloud Datastore, etc)")
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID, option.WithCredentialsFile("/usr/local/google/home/ruihuang/Downloads/ruihuang-bookshelf01-4a4ea6668b1f.json"))
	if err != nil {
		return nil, err
	}

	// Create the topic if it doesn't exist.
	if exists, err := client.Topic(PubsubTopicID).Exists(ctx); err != nil {
		return nil, err
	} else if !exists {
		if _, err := client.CreateTopic(ctx, PubsubTopicID); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func configureOAuthClient(clientID, clientSecret string) *oauth2.Config {
	redirectURL := os.Getenv("OAUTH2_CALLBACK")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/oauth2callback"
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

type cloudSQLConfig struct {
	Username, Password, Instance string
}

func configureCloudSQL(config cloudSQLConfig) (BookDatabase, error) {
	fmt.Printf("\nconfigureCloudSQL! inside\n")
	if os.Getenv("GAE_INSTANCE") != "" {
		// Running in production.
		fmt.Printf("\nconfigureCloudSQL! GAE_INSTANCE\n")
		return newMySQLDB(MySQLConfig{
			Username:   config.Username,
			Password:   config.Password,
			UnixSocket: "/cloudsql/" + config.Instance,
		})
	}

	// Running locally.
	return newMySQLDB(MySQLConfig{
		Username: config.Username,
		Password: config.Password,
		Host:     "localhost",
		Port:     3306,
	})
}
