//
// Wialongo
// A port of a PHP library for Wialon Remote API
// Currently, It's under development and not production ready
//
// @Information(alekum 28/04/2019): Wialon Remote API Reference
// @see https://sdk.wialon.com/wiki/en/sidebar/remoteapi/apiref/apiref
//
// Copyright (c) 2019, alekum
//

package wialongo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// WialonError : type aliasing
type WialonError int16

// WialonResult : type aliasing
type WialonResult string

// WialonAPISvc : actual api call name of the Wialon ApI, @see https://sdk.wialon.com/wiki/en/sidebar/remoteapi/apiref/reqformat/reqformat
type WialonAPISvc string

// WialonToken : specify type for wialon api token
type WialonToken string

// WialonAPIParams : type aliasing to map[string]string
// cause most of the time it's just a map(that will be serialized into json object)
type WialonAPIParams map[string]interface{}

// WialonAPI : type aliasing(interface) of wialon struct
type WialonAPI interface {
	Logout() (WialonResult, bool) // re-check perhaps it's better return a json object
	Login(token string) (WialonResult, bool)
	UpdateExtraParams(params WialonAPIParams)
	WialonAPICall(action WialonAPISvc, args WialonAPIParams) WialonResult
}

// Wialon : base struct of Wialon API
type Wialon struct {
	Sid           string
	BaseAPIUrl    string
	DefaultParams WialonAPIParams
}

// WialonErrors
const (
	InvalidSession WialonError = iota + 1
	InvalidService
	InvalidResult
	InvalidInput
	ErrPerformReq
	UnknownError
	AccessDenied
	InvalidUserNameOrPassword
	AuthServerUnavailable
	NoMsgForSelectedInterval WialonError = 1001
	ItemAlreadyExists        WialonError = 1002
	OneReqAllowed            WialonError = 1003
	MsgLimitExceeded         WialonError = 1004
	ExecutionTimeExceeded    WialonError = 1005
	LimitAttemptsTwoFactAuth WialonError = 1006
	IPChangedOrSessExpired   WialonError = 1011
	UserCannotBeBoundToAcc   WialonError = 2014
	SensDeletingForbidden    WialonError = 2015
)

var (
	// WialonErrors : a map of descriptions for the WialonError consts
	WialonErrors = map[WialonError]string{
		InvalidSession:            "Invalid session",
		InvalidService:            "Invalid service",
		InvalidResult:             "Invalid result",
		InvalidInput:              "Invalid input",
		ErrPerformReq:             "Error performing request",
		UnknownError:              "Unknown error",
		AccessDenied:              "Access denied",
		InvalidUserNameOrPassword: "Invalid user name or password",
		AuthServerUnavailable:     "Authorization server is unavailable, please try again later",
		NoMsgForSelectedInterval:  "No message for selected interval",
		ItemAlreadyExists:         "Item with such inique property already exists",
		OneReqAllowed:             "Only one request of given time is allowed at the moment",
		MsgLimitExceeded:          "Limit of messages has been exceeded",
		ExecutionTimeExceeded:     "Execution time has exceeded the limit",
		LimitAttemptsTwoFactAuth:  "Exceeding the limit of attempts to enter a two-factor authorization code",
		IPChangedOrSessExpired:    "Your IP has changed or session has expired",
		UserCannotBeBoundToAcc:    "Selected user is a creator for some system objects, thus this user cannot be bound to a new account",
		SensDeletingForbidden:     "Sensor deleting is forbidden because of using in another sensor or advanced properties of the unit",
	}
)

func (we WialonError) String() string {
	return fmt.Sprintf("%d : %s", we, WialonErrors[we])
}

// NewDefaultWialon : create Wialon object with default parameters
func NewDefaultWialon() *Wialon {
	return NewWialon("https", "hst-api.wialon.com", "", "", map[string]interface{}{})
}

// NewWialon : create Wialon object to work with WialonAPI
func NewWialon(scheme string, host string, port string, sid string, extraParams map[string]interface{}) *Wialon {
	w := new(Wialon)

	if port != "" {
		port = ":" + port
	}

	w.Sid = sid
	w.BaseAPIUrl = fmt.Sprintf("%s://%s%s/wialon/ajax.html?", scheme, host, port)
	w.DefaultParams = extraParams
	return w
}

// Login : login into wialon and get sid
func (w *Wialon) Login(token WialonToken) (WialonResult, bool) {
	var result WialonResult
	data := map[string]interface{}{
		"token": string(token), // it will be url encoded later in api call
	}
	result = w.WialonAPICall("token_login", data)

	var jsonResult = WialonAPIParams{}
	json.Unmarshal([]byte(result), &jsonResult)

	// @Information(alekum 28/04/2019): if eid somewhere in the future is not a string we will get an empty sid every time
	if eid, ok := jsonResult["eid"].(string); ok {
		w.Sid = eid
		return result, true
	}
	return result, false
}

// Logout : logout from wialon and discard sid
func (w *Wialon) Logout() (WialonResult, bool) {
	result := w.WialonAPICall("core_logout", WialonAPIParams{})

	var jsonResult = map[string]interface{}{}
	json.Unmarshal([]byte(result), &jsonResult)

	if err, ok := jsonResult["error"].(float64); ok && int(err) == 0 {
		w.Sid = ""
		return result, true
	}
	return result, false
}

// UpdateExtraParams : resolve some parameters for SVC command, replace current default params
func (w *Wialon) UpdateExtraParams(params WialonAPIParams) {
	for k, v := range params {
		w.DefaultParams[k] = v
	}
}

// WialonAPICall : actual call of the remote wialon api, It's the same as a call function in php lib
func (w *Wialon) WialonAPICall(action WialonAPISvc, params WialonAPIParams) WialonResult {
	svc := string(action)
	contentType := "application/x-www-form-urlencoded"
	if strings.HasPrefix(svc, "unit_group") {
		svc = svc[0:len("unit_group")] + "/" + svc[len("unit_group")+1:]
	} else {
		svc = strings.Replace(svc, "_", "/", 1)
	}

	// @Cleanup(alekum 28/04/2019):
	// Perhaps manual json marshalling is a better way to do that due to natura of json-like data structures
	reqParams := WialonAPIParams{
		"sid":    w.Sid,
		"svc":    svc,
		"params": params,
	}
	w.UpdateExtraParams(reqParams)

	u, err := url.Parse(w.BaseAPIUrl)
	if err != nil {
		panic("Cannot parse Wialon baseAPIUrl")
	}

	urlValues := url.Values{}
	for k, v := range w.DefaultParams {
		switch val := v.(type) {
		case string:
			// @Cleanup(alekum 29/04/2019):
			// All of a sudden, the problem with different value types popped up
			// where I must be processing them differently. So, It leaves a room in
			// the future refactoring to make the marshaling more robust!!
			urlValues.Set(k, val)
		case int:
			urlValues.Set(k, strconv.Itoa(val))
		case float32, float64:
			urlValues.Set(k, fmt.Sprintf("%g", val))
		default:
			jsonified, err := json.Marshal(val)
			if err != nil {
				panic("Cannot Marshal params")
			}
			urlValues.Set(k, string(jsonified))
		}
	}
	u.RawQuery = urlValues.Encode()

	fmt.Println(fmt.Sprintf("\n\t> APIUrl: %s,\n\tCType: %s,\n\tQParams: %s,\n\tMapParams: %s\n", u.Hostname()+u.Path, contentType, u.RawQuery, w.DefaultParams))
	response, err := http.Post(w.BaseAPIUrl, contentType, bytes.NewBufferString(u.RawQuery))
	if err != nil {
		panic("Cannot do Post Query")
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic("Cannot read response body")
	}
	return WialonResult(responseBody)
}

// ErrorHandler : a helper function to deal with wialon erros
func ErrorHandler(res WialonResult) (we WialonError, msg string) {
	var errResult = map[string]interface{}{}
	json.Unmarshal([]byte(res), &errResult)
	loginError, _ := errResult["error"].(float64)
	we = WialonError(int(loginError))
	msg = WialonErrors[we]
	return we, msg
}
