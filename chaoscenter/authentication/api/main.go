package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	response "github.com/litmuschaos/litmus/chaoscenter/authentication/api/handlers"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/authConfig"
	"google.golang.org/grpc/credentials"

	grpcHandler "github.com/litmuschaos/litmus/chaoscenter/authentication/api/handlers/grpc"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/api/middleware"
	grpcPresenter "github.com/litmuschaos/litmus/chaoscenter/authentication/api/presenter/protos"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/api/routes"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/entities"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/misc"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/project"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/services"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/session"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/user"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/utils"

	"google.golang.org/grpc"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	JwtSecret      string   `required:"true" split_words:"true"`
	AdminUsername  string   `required:"true" split_words:"true"`
	AdminPassword  string   `required:"true" split_words:"true"`
	DbServer       string   `required:"true" split_words:"true"`
	DbUser         string   `required:"true" split_words:"true"`
	DbPassword     string   `required:"true" split_words:"true"`
	AllowedOrigins []string `split_words:"true" default:"^(http://|https://|)litmuschaos.io(:[0-9]+|)?,^(http://|https://|)localhost(:[0-9]+|)"`
}

var config Config

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	printVersion()
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err)
	}
}

// @title Chaoscenter API documentation
func main() {
	// send logs to stderr, so we can use 'kubectl logs'
	_ = flag.Set("logtostderr", "true")
	_ = flag.Set("v", "3")

	flag.Parse()

	client, err := utils.MongoConnection()
	if err != nil {
		log.Fatal("database connection error $s", err)
	}

	db := client.Database(utils.DBName)

	// Creating User Collection
	err = utils.CreateCollection(utils.UserCollection, db)
	if err != nil {
		log.Errorf("failed to create collection  %s", err)
	}

	err = utils.CreateIndex(utils.UserCollection, utils.UsernameField, db)
	if err != nil {
		log.Errorf("failed to create index  %s", err)
	}

	// Creating Project Collection
	err = utils.CreateCollection(utils.ProjectCollection, db)
	if err != nil {
		log.Errorf("failed to create collection  %s", err)
	}

	// Creating AuthConfig Collection
	err = utils.CreateCollection(utils.AuthConfigCollection, db)
	if err != nil {
		log.Errorf("failed to create collection  %s", err)
	}

	// Creating RevokedToken Collection
	if err = utils.CreateCollection(utils.RevokedTokenCollection, db); err != nil {
		log.Errorf("failed to create collection  %s", err)
	}

	if err = utils.CreateTTLIndex(utils.RevokedTokenCollection, db); err != nil {
		log.Errorf("failed to create index  %s", err)
	}

	// Creating ApiToken Collection
	if err = utils.CreateCollection(utils.ApiTokenCollection, db); err != nil {
		log.Errorf("failed to create collection  %s", err)
	}

	userCollection := db.Collection(utils.UserCollection)
	userRepo := user.NewRepo(userCollection)

	projectCollection := db.Collection(utils.ProjectCollection)
	projectRepo := project.NewRepo(projectCollection)

	revokedTokenCollection := db.Collection(utils.RevokedTokenCollection)
	revokedTokenRepo := session.NewRevokedTokenRepo(revokedTokenCollection)

	apiTokenCollection := db.Collection(utils.ApiTokenCollection)
	apiTokenRepo := session.NewApiTokenRepo(apiTokenCollection)

	authConfigCollection := db.Collection(utils.AuthConfigCollection)
	authConfigRepo := authConfig.NewAuthConfigRepo(authConfigCollection)

	miscRepo := misc.NewRepo(db, client)

	applicationService := services.NewService(userRepo, projectRepo, miscRepo, revokedTokenRepo, apiTokenRepo, authConfigRepo, db)

	err = response.AddSalt(applicationService)
	if err != nil {
		log.Fatal("couldn't create salt $s", err)
	}

	validatedAdminSetup(applicationService)

	if utils.EnableInternalTls {
		if utils.TlsCertPath != "" && utils.TlSKeyPath != "" {
			go runGrpcServerWithTLS(applicationService)
		} else {
			log.Fatalf("Failure to start chaoscenter authentication GRPC server due to empty TLS cert file path and TLS key path")
		}
	} else {
		go runGrpcServer(applicationService)
	}

	runRestServer(applicationService)
}

func validatedAdminSetup(service services.ApplicationService) {
	// Assigning UID to admin
	uID := uuid.Must(uuid.NewRandom()).String()

	// Generating password hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(utils.AdminPassword), utils.PasswordEncryptionCost)
	if err != nil {
		log.Println("Error generating password for admin")
	}
	password := string(hashedPassword)

	adminUser := entities.User{
		ID:             uID,
		Username:       utils.AdminName,
		Password:       password,
		Role:           entities.RoleAdmin,
		IsInitialLogin: true,
		Audit: entities.Audit{
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		},
	}

	_, err = service.CreateUser(&adminUser)
	if err != nil && err == utils.ErrUserExists {
		log.Println("Admin already exists in the database, not creating a new admin")
	} else if err != nil {
		log.Fatalf("Unable to create admin, error: %v", err)
	}
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func runRestServer(applicationService services.ApplicationService) {
	// Starting REST server using Gin
	gin.SetMode(gin.ReleaseMode)
	gin.EnableJsonDecoderDisallowUnknownFields()
	app := gin.Default()
	app.Use(middleware.ValidateCors(config.AllowedOrigins))
	// Enable dex routes only if passed via environment variables
	if utils.DexEnabled {
		routes.DexRouter(app, applicationService)
	}
	routes.CapabilitiesRouter(app)
	routes.MiscRouter(app, applicationService)
	routes.UserRouter(app, applicationService)
	routes.ProjectRouter(app, applicationService)

	log.Infof("Listening and serving HTTP on %s", utils.RestPort)

	if utils.EnableInternalTls {
		if utils.TlsCertPath != "" && utils.TlSKeyPath != "" {
			conf := utils.GetTlsConfig()
			server := http.Server{
				Addr:      utils.RestPort,
				Handler:   app,
				TLSConfig: conf,
			}
			log.Infof("Listening and serving HTTPS on %s", utils.RestPort)
			err := server.ListenAndServeTLS("", "")
			if err != nil {
				log.Fatalf("Failure to start litmus-portal authentication REST server due to %v", err)
			}
		} else {
			log.Fatalf("Failure to start chaoscenter authentication REST server due to empty TLS cert file path and TLS key path")
		}
	} else {
		log.Infof("Listening and serving HTTP on %s", utils.RestPort)
		err := app.Run(utils.RestPort)
		if err != nil {
			log.Fatalf("Failure to start litmus-portal authentication REST server due to %v", err)
		}
	}
}

func runGrpcServer(applicationService services.ApplicationService) {
	// Starting gRPC server
	lis, err := net.Listen("tcp", utils.GrpcPort)
	if err != nil {
		log.Fatalf("Failure to start litmus-portal authentication server due"+
			" to %s", err)
	}
	grpcApplicationServer := grpcHandler.ServerGrpc{ApplicationService: applicationService}
	grpcServer := grpc.NewServer()
	grpcPresenter.RegisterAuthRpcServiceServer(grpcServer, &grpcApplicationServer)
	log.Infof("Listening and serving gRPC on %s", utils.GrpcPort)
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("Failure to start chaoscenter authentication GRPC server due to %v", err)
	}
}

func runGrpcServerWithTLS(applicationService services.ApplicationService) {

	// Starting gRPC server
	lis, err := net.Listen("tcp", utils.GrpcPort)
	if err != nil {
		log.Fatalf("Failure to start litmus-portal authentication server due to %s", err)
	}

	// configuring TLS config based on provided certificates & keys
	conf := utils.GetTlsConfig()

	// create tls credentials
	tlsCredentials := credentials.NewTLS(conf)

	// create grpc server with tls credential
	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))

	grpcApplicationServer := grpcHandler.ServerGrpc{ApplicationService: applicationService}

	grpcPresenter.RegisterAuthRpcServiceServer(grpcServer, &grpcApplicationServer)

	log.Infof("Listening and serving gRPC on %s with TLS", utils.GrpcPort)
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("Failure to start chaoscenter authentication GRPC server due to %v", err)
	}
}
