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
	db             database.Database
	apiMutex       sync.Mutex
	certificate    string
	keyfile        string
	apiIP          string
	apiPort        string
	apiPassword    string
	browsingFolder string
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
		db:             db,
		certificate:    conf.ExternalAPI.CertPath,
		keyfile:        conf.ExternalAPI.KeyPath,
		apiIP:          conf.ExternalAPI.IP,
		apiPassword:    conf.ExternalAPI.Password,
		apiPort:        conf.ExternalAPI.Port,
		browsingFolder: conf.ExternalAPI.BrowsingFolder,
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
	functions := []string{apiV1 + "/status/consumptions",
		apiV1 + "/status/consumption/leds",
		apiV1 + "/status/consumption/blinds",
		apiV1 + "/status/consumption/hvacs"}
	apiInfo := APIFunctions{
		Functions: functions,
	}
	inrec, _ := json.MarshalIndent(apiInfo, "", "  ")
	w.Write(inrec)
}

type APIV1SwitchConsumptions struct {
	TotalPower    int `json:"totalPower"`
	LightingPower int `json:"lightningPower"`
	BlindPower    int `json:"blindPower"`
	HvacPower     int `json:"hvacPower"`
}

func (api *API) getV1Consumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	power := 0
	leds := 0
	blinds := 0
	hvacs := 0

	for _, driver := range database.GetStatusBlinds(api.db) {
		power += driver.LinePower
		blinds += driver.LinePower
	}

	for _, driver := range database.GetStatusLeds(api.db) {
		power += driver.LinePower
		leds += driver.LinePower
	}

	conso := APIV1SwitchConsumptions{
		TotalPower:    power,
		LightingPower: leds,
		BlindPower:    blinds,
		HvacPower:     hvacs,
	}
	inrec, _ := json.MarshalIndent(conso, "", "  ")
	w.Write(inrec)
}

type APIV1Consumption struct {
	Power int `json:"power"`
}

func (api *API) getV1LightingConsumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	power := 0

	for _, driver := range database.GetStatusLeds(api.db) {
		power += driver.LinePower
	}

	conso := APIV1Consumption{
		Power: power,
	}
	inrec, _ := json.MarshalIndent(conso, "", "  ")
	w.Write(inrec)
}

func (api *API) getV1BlindConsumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	power := 0

	for _, driver := range database.GetStatusBlinds(api.db) {
		power += driver.LinePower
	}

	conso := APIV1Consumption{
		Power: power,
	}
	inrec, _ := json.MarshalIndent(conso, "", "  ")
	w.Write(inrec)
}

func (api *API) getV1HVACConsumptions(w http.ResponseWriter, req *http.Request) {
	api.setDefaultHeader(w)
	power := 0

	conso := APIV1Consumption{
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
	router.HandleFunc(apiV1+"/status/consumption/leds", api.getV1LightingConsumptions).Methods("GET")
	router.HandleFunc(apiV1+"/status/consumption/blinds", api.getV1BlindConsumptions).Methods("GET")
	router.HandleFunc(apiV1+"/status/consumption/hvacs", api.getV1HVACConsumptions).Methods("GET")

	//unversionned API
	router.HandleFunc("/versions", api.getAPIs).Methods("GET")
	router.HandleFunc("/functions", api.getFunctions).Methods("GET")

	if api.browsingFolder != "" {
		sh2 := http.StripPrefix("/", http.FileServer(http.Dir(api.browsingFolder)))
		router.PathPrefix("/").Handler(sh2)
	}

	log.Fatal(http.ListenAndServeTLS(api.apiIP+":"+api.apiPort, api.certificate, api.keyfile, router))
}
