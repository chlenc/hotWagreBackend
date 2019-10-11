package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
	"github.com/wavesplatform/go-lib-crypto"
	"log"
	"net/http"
	"os"
)

type App struct {
	db *sql.DB
}

type TUser struct {
	Id      string `json:"id"`
	Seed    string `json:"seed"`
	Address string `json:"address"`
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
		app.db.QueryRow("select * from users where id = $1", id).Scan(&user.Id, &user.Seed, &user.Address)
		log.Println(id, user)
		if user.Seed == "" || user.Address == "" {
			wavesCrypto := wavesplatform.NewWavesCrypto()
			seed := wavesCrypto.RandomSeed()
			address := wavesCrypto.AddressFromSeed(seed, wavesplatform.TestNet)
			user.Id = id
			user.Address = string(address)
			user.Seed = string(seed)
			sqlStatement := `INSERT INTO users (id, seed, address) VALUES ($1, $2, $3) RETURNING id`
			err := app.db.QueryRow(sqlStatement, user.Id, user.Seed, user.Address).Scan(&id)
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

func render(c *gin.Context, data gin.H) {

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS,POST,PUT")
	c.Header("Access-Control-Allow-Headers", "Access-Control-Allow-Headers, Origin,Accept, X-Requested-With, Content-Type, Access-Control-Request-Method, Access-Control-Request-Headers")

	c.JSON(http.StatusOK, data["payload"])
}
