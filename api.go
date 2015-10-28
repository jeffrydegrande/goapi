package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
)

type API struct {
	Version        string          `json:"_version"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Metadata       []Metadata      `json:"metadata"`
	ResourceGroups []ResourceGroup `json:"resourceGroups"`
	Content        []Content       `json:"content"`
}

type Host struct {
	Value string `json:"value"`
}

type Format struct {
	Value string `json:"value"`
}

type Metadata struct {
	Format Format `json:"FORMAT"`
	Host   Host   `json:"HOST"`
}

type ResourceGroup struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Resources   []Resource `json:"resources"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Model struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Headers     []Header `json:"headers"`
	Body        string   `json:"body"`
	Schema      string   `json:"schema"`
}

type Parameter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default"`
	Example     string   `json:"example"`
	Values      []string `json:"values"`
}

type Example struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Requests    []Request  `json:"requests"`
	Responses   []Response `json:"responses"`
}

type Request struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Headers     []Header `json:"headers"`
	Body        string   `json:"body"`
	Schema      string   `json:"schema"`
}

type Response struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Headers     []Header `json:"headers"`
	Body        string   `json:"body"`
	Schema      string   `json:"schema"`
}

type Action struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Method      string      `json:"method"`
	Parameters  []Parameter `json:"parameters"`
	Headers     []Header    `json:"headers"`
	Examples    []Example   `json:"examples"`
}

type Resource struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	UriTemplate string      `json:"uriTemplate"`
	Model       Model       `json:"model"`
	Parameters  []Parameter `json:"parameters"`
	Headers     []Header    `json:"headers"`
	Actions     []Action    `json:"actions"`
}

type VariableName struct {
	Literal  string `json:"name"`
	Variable bool   `json:"variable"`
}

type TypeSpecification struct {
	Name        string         `json:"name"`
	NestedTypes []VariableName `json:"nestedTypes"`
}

type TypeDefinition struct {
}

type Content struct {
	Element string       `json:"element"`
	Name    VariableName `json:"name"`
}

type Question struct {
	Answers map[string]string
}

func parseJSON(r io.Reader) (*API, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	api := new(API)
	err = json.Unmarshal(b, &api)
	if err != nil {
		return nil, err
	}

	return api, nil
}

func parseMarkdown(r io.Reader) ([]byte, error) {
	path, err := exec.LookPath("drafter")
	if err != nil {
		return nil, errors.New("Couldn't find drafter. Please install it first https://github.com/apiaryio/drafter")
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	echo := exec.Command("echo", string(b))
	out, err := echo.StdoutPipe()
	if err != nil {
		return nil, err
	}

	echo.Start()

	cmd := exec.Command(path, "--format", "json")
	cmd.Stdin = out
	return cmd.Output()
}

func NewAPI(filename string) (*API, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	json, err := parseMarkdown(f)

	if err != nil {
		fmt.Printf("Markdown is not valid\n")
		return nil, err
	}

	api, err := parseJSON(bytes.NewBuffer(json))
	if err != nil {
		fmt.Printf("Json is not valid\n")
	}
	return api, nil
}

func (api *API) GenerateRoutes(router *mux.Router, cors bool, stickToHappyPath bool) error {

	for _, group := range api.ResourceGroups {
		for _, resource := range group.Resources {
			for _, action := range resource.Actions {
				for _, example := range action.Examples {
					handler := func(w http.ResponseWriter, r *http.Request) {
						reply := example.Responses[0]

						if len(example.Responses) > 1 && !stickToHappyPath {
							responses := make(map[string]string, len(example.Responses))

							for _, response := range example.Responses {
								responses[response.Name] = response.Body
							}

							fmt.Println("Asking Question")
							askingQuestions <- &Question{Answers: responses}

							fmt.Println("Waiting for response")
							text := <-responseChan
							if text != "" {
								for _, response := range example.Responses {
									if text == response.Name {
										reply = response
										break
									}
								}
							}
						}

						w.Header().Set("Access-Control-Allow-Origin", "*")
						for _, header := range reply.Headers {
							w.Header().Set(header.Name, header.Value)
						}

						httpStatus, _ := strconv.Atoi(reply.Name)
						w.WriteHeader(httpStatus)

						var isReferenceToModel = regexp.MustCompile(`\[.*\]\[\]`)

						if isReferenceToModel.MatchString(reply.Body) {
							fmt.Println("sending", resource.Model.Body)
							w.Write([]byte(resource.Model.Body))
						} else {
							fmt.Println("sending", reply.Body)
							w.Write([]byte(reply.Body))
						}
					}

					fmt.Println(resource.UriTemplate)

					router.HandleFunc(resource.UriTemplate, handler).
						Methods(action.Method)

					if cors {
						preflight := func(w http.ResponseWriter, r *http.Request) {
							allowHeaders := r.Header.Get("Access-Control-Request-Headers")
							w.Header().Set("Access-Control-Allow-Origin", "*")
							w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
							w.WriteHeader(201)
						}

						router.HandleFunc(resource.UriTemplate, preflight).
							Methods("OPTIONS")
					}
				}
			}
		}
	}
	return nil
}
