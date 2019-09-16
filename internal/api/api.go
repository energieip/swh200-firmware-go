package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/energieip/common-components-go/pkg/dswitch"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/gorilla/mux"
)

type API struct {
	db             database.Database
	certificate    string
	keyfile        string
	apiIP          string
	apiPort        string
	apiPassword    string
	browsingFolder string
	consumption    *dswitch.SwitchConsumptions
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
func InitAPI(db database.Database, conf pkg.ServiceConfig, conso *dswitch.SwitchConsumptions) *API {
	api := API{
		db:             db,
		certificate:    conf.ExternalAPI.CertPath,
		keyfile:        conf.ExternalAPI.KeyPath,
		apiIP:          conf.ExternalAPI.IP,
		apiPassword:    conf.ExternalAPI.Password,
		apiPort:        conf.ExternalAPI.Port,
		browsingFolder: conf.ExternalAPI.BrowsingFolder,
		consumption:    conso,
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

func (api *API) getV1Consumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	inrec, _ := json.MarshalIndent(api.consumption, "", "  ")
	w.Write(inrec)
}

type APIV1Consumption struct {
	Power int `json:"power"`
}

func (api *API) swagger() {
	router := mux.NewRouter()
	sh := http.StripPrefix("/swaggerui/", http.FileServer(http.Dir("/data/www/swaggerui/")))
	router.PathPrefix("/swaggerui/").Handler(sh)

	// API v1.0
	apiV1 := "/v1.0"
	router.HandleFunc(apiV1+"/functions", api.getV1Functions).Methods("GET")

	//status
	router.HandleFunc(apiV1+"/status/consumptions", api.getV1Consumptions).Methods("GET")

	//unversionned API
	router.HandleFunc("/versions", api.getAPIs).Methods("GET")
	router.HandleFunc("/functions", api.getFunctions).Methods("GET")

	if api.browsingFolder != "" {
		sh2 := http.StripPrefix("/", http.FileServer(http.Dir(api.browsingFolder)))
		router.PathPrefix("/").Handler(sh2)
	}

	log.Fatal(http.ListenAndServeTLS(api.apiIP+":"+api.apiPort, api.certificate, api.keyfile, router))
}
