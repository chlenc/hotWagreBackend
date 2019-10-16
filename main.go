package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
	"github.com/wavesplatform/go-lib-crypto"
	"github.com/wavesplatform/gowaves/pkg/client"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const DAPP_ADDRESS = "3NCKMaA2EQ8AwbB73pPNDyidJxqjTcPYmLh"

type App struct {
	db *sql.DB
}

type TUser struct {
	Id         string `json:"id"`
	Seed       string `json:"seed"`
	Address    string `json:"address"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

func main() {

	e := godotenv.Load()
	if e != nil {
		fmt.Print(e)
	}

	username := os.Getenv("db_user")
	password := os.Getenv("db_pass")
	dbName := os.Getenv("db_name")
	dbHost := os.Getenv("db_host")

	dbUri := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password=%s", dbHost, username, dbName, password)
	fmt.Println(dbUri)

	//db, err := sql.Open("postgres", dbUri)
	db, err := sql.Open("postgres", "host=localhost user=postgres dbname=postgres port="+"5432"+" sslmode=disable")
	db.SetMaxOpenConns(10)
	if err != nil {
		log.Println("failed to connect database", err)
		return
	}

	var app = App{db}

	r := gin.Default()
	r.Use(gin.Logger())

	app.initializeRoutes(r)

	r.Run()
}

func (app *App) initializeRoutes(r *gin.Engine) {
	apiRoutes := r.Group("/api")
	{
		apiRoutes.GET("/login", app.login)
	}

	r.NoRoute(func(c *gin.Context) {
		render(c, gin.H{"payload": "not found"})
	})

}

func (app *App) login(c *gin.Context) {

	id, isId := c.GetQuery("id")
	log.Println(id)

	if isId {
		user := &TUser{}
		app.db.QueryRow("select * from users where id = $1", id).Scan(
			&user.Id, &user.Seed, &user.Address, &user.PublicKey, &user.PrivateKey)
		log.Println(id, user)
		if user.Seed == "" || user.Address == "" {
			wavesCrypto := wavesplatform.NewWavesCrypto()
			seed := wavesCrypto.RandomSeed()

			user.Id = id
			user.Address = string(wavesCrypto.AddressFromSeed(seed, wavesplatform.TestNet))
			user.Seed = string(seed)
			user.PublicKey = string(wavesCrypto.PublicKey(seed))
			user.PrivateKey = string(wavesCrypto.PrivateKey(seed))

			sqlStatement := `INSERT INTO users (id, seed, address,publicKey,privateKey) VALUES ($1, $2, $3, $4, $5) RETURNING id`
			err := app.db.QueryRow(sqlStatement, user.Id, user.Seed, user.Address, user.PublicKey, user.PrivateKey).Scan(&id)
			if err != nil {
				c.JSON(http.StatusBadRequest, "database error, err: "+err.Error())
				return
			}
		}
		render(c, gin.H{"payload": user})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"payload": "invalid id"})
	}
}

func (app *App) bet(c *gin.Context) {

	id, isId := c.GetQuery("id")
	eventStr, isEvent := c.GetQuery("event")

	if !(isId && isEvent) {
		c.JSON(http.StatusBadRequest, gin.H{"payload": "error"})
		return
	}

	event, err := strconv.ParseInt(eventStr, 10, 64)

	if err != nil || (event != 1 && event != 2) {
		c.JSON(http.StatusBadRequest, gin.H{"payload": "error"})
		return
	}

	user := &TUser{}
	app.db.QueryRow("select * from users where id = $1", id).Scan(
		&user.Id, &user.Seed, &user.Address, &user.PublicKey, &user.PrivateKey)
	log.Println(id, user)
	if user.Seed == "" {
		c.JSON(http.StatusBadRequest, gin.H{"payload": "error"})
		return
	}

	// Create sender's public key from BASE58 string
	sender, err := crypto.NewPublicKeyFromBase58(user.PublicKey)
	if err != nil {
		panic(err)
	}

	// Create sender's private key from BASE58 string
	sk, err := crypto.NewSecretKeyFromBase58(user.PrivateKey)
	if err != nil {
		panic(err)
	}

	// Create script's address
	a, err := proto.NewAddressFromString(DAPP_ADDRESS)
	if err != nil {
		panic(err)
	}

	// Create Function Call that will be passed to the script
	fc := proto.FunctionCall{Name: "bet", Arguments: proto.Arguments{proto.IntegerArgument{Value: event}}}

	// Current time in milliseconds
	ts := time.Now().Unix() * 1000

	// Fee asset is Waves
	waves := proto.OptionalAsset{Present: false}

	// New InvokeScript Transaction
	tx := proto.NewUnsignedInvokeScriptV1(
		'T',
		sender,
		proto.Recipient{Address: &a},
		fc,
		proto.ScriptPayments{proto.ScriptPayment{Amount: 1e8, Asset: waves}},
		waves,
		500000,
		uint64(ts),
	)

	// Sing the transaction with the private key
	err = tx.Sign(sk)
	if err != nil {
		panic(err)
	}
	// Create new HTTP client to send the transaction to public TestNet nodes
	client, err := client.NewClient(client.Options{BaseUrl: "https://testnodes.wavesnodes.com", Client: &http.Client{}})
	if err != nil {
		panic(err)
	}

	// Context to cancel the request execution on timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send the transaction to the network
	_, err = client.Transactions.Broadcast(ctx, tx)
	if err != nil {
		panic(err)
	}
}

func render(c *gin.Context, data gin.H) {

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS,POST,PUT")
	c.Header("Access-Control-Allow-Headers", "Access-Control-Allow-Headers, Origin,Accept, X-Requested-With, Content-Type, Access-Control-Request-Method, Access-Control-Request-Headers")

	c.JSON(http.StatusOK, data["payload"])
}
