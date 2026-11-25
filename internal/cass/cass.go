// Package cass opens a gocql session for Cassandra or Amazon Keyspaces (config only).
package cass

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/sigv4"
	"github.com/gocql/gocql"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// Connect uses CASSANDRA_HOSTS, CASSANDRA_KEYSPACE, CASSANDRA_AUTH=none|sigv4,
// CASSANDRA_TLS, CASSANDRA_TLS_CA and CASSANDRA_CONSISTENCY.
func Connect() (*gocql.Session, error) {
	cl := gocql.NewCluster(strings.Split(env("CASSANDRA_HOSTS", "localhost"), ",")...)
	cl.Keyspace = env("CASSANDRA_KEYSPACE", "bank")
	cl.Timeout = 5 * time.Second
	cl.ConnectTimeout = 10 * time.Second
	cl.SerialConsistency = gocql.LocalSerial
	var lvl gocql.Consistency
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(env("CASSANDRA_CONSISTENCY", "LOCAL_QUORUM")))); err != nil {
		return nil, fmt.Errorf("consistency: %w", err)
	}
	cl.Consistency = lvl
	if env("CASSANDRA_TLS", "false") == "true" {
		cl.Port = 9142
		cl.DisableInitialHostLookup = true // Keyspaces exposes one endpoint
		cl.NumConns = 3
		cl.SslOpts = &gocql.SslOptions{CaPath: env("CASSANDRA_TLS_CA", ""), EnableHostVerification: true}
	}
	if env("CASSANDRA_AUTH", "none") == "sigv4" {
		a := sigv4.NewAwsAuthenticator()
		a.Region = env("AWS_REGION", "ap-southeast-2")
		cl.Authenticator = a
	}
	var s *gocql.Session
	var err error
	for i := 0; i < 30; i++ { // wait for the database on cold start
		if s, err = cl.CreateSession(); err == nil {
			return s, nil
		}
		time.Sleep(3 * time.Second)
	}
	return nil, err
}
