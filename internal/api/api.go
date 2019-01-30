package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/gorilla/mux"
)

type API struct {
	db          database.Database
	apiMutex    sync.Mutex
	installMode *bool
	certificate string
	keyfile     string
}

type APIInfo struct {
	Versions []string `json:"versions"`
}

type APIFunctions struct {
	Functions []string `json:"functions"`
}

func (api *API) getAPIs(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	versions := []string{"v1.0"}
	apiInfo := APIInfo{
		Versions: versions,
	}
	inrec, _ := json.MarshalIndent(apiInfo, "", "  ")
	w.Write(inrec)
}

//InitAPI start API connection
func InitAPI(db database.Database, conf pkg.ServiceConfig) *API {
	api := API{
		db:          db,
		certificate: conf.Certificate,
		keyfile:     conf.Key,
	}
	go api.swagger()
	return &api
}

func (api *API) setDefaultHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func (api *API) getFunctions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	functions := []string{"/versions"}
	apiInfo := APIFunctions{
		Functions: functions,
	}
	inrec, _ := json.MarshalIndent(apiInfo, "", "  ")
	w.Write(inrec)
}

func (api *API) getV1Functions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	apiV1 := "/v1.0"
	functions := []string{apiV1 + "/status/consumptions"}
	apiInfo := APIFunctions{
		Functions: functions,
	}
	inrec, _ := json.MarshalIndent(apiInfo, "", "  ")
	w.Write(inrec)
}

type APIV1Consumptions struct {
	Power int `json:"power"`
}

func (api *API) getV1Consumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	power := 0

	for _, driver := range database.GetStatusBlinds(api.db) {
		power += driver.LinePower
	}

	for _, driver := range database.GetStatusLeds(api.db) {
		power += driver.LinePower
	}

	conso := APIV1Consumptions{
		Power: power,
	}
	inrec, _ := json.MarshalIndent(conso, "", "  ")
	w.Write(inrec)
}

func (api *API) swagger() {
	router := mux.NewRouter()
	sh := http.StripPrefix("/swaggerui/", http.FileServer(http.Dir("/media/userdata/www/swaggerui/")))
	router.PathPrefix("/swaggerui/").Handler(sh)

	// API v1.0
	apiV1 := "/v1.0"
	router.HandleFunc(apiV1+"/functions", api.getV1Functions).Methods("GET")

	//status
	router.HandleFunc(apiV1+"/status/consumptions", api.getV1Consumptions).Methods("GET")

	//unversionned API
	router.HandleFunc("/versions", api.getAPIs).Methods("GET")
	router.HandleFunc("/functions", api.getFunctions).Methods("GET")

	log.Fatal(http.ListenAndServeTLS(":8888", api.certificate, api.keyfile, router))
}
