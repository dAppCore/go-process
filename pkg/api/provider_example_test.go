package api_test

import (
	. "dappco.re/go"
	process "dappco.re/go/process"
	processapi "dappco.re/go/process/pkg/api"
	"github.com/gin-gonic/gin"
)

func exampleProvider() *processapi.ProcessProvider {
	raw, _ := process.NewService(process.Options{})(New())
	svc := raw.(*process.Service)
	reg := process.NewRegistry(PathJoin(TempDir(), "go-process-provider-"+ID()))
	return processapi.NewProvider(reg, svc, nil)
}

func ExampleNewProvider() {
	provider := exampleProvider()
	Println(provider.Name())
	// Output: process
}

func ExampleProcessProvider_Name() {
	provider := exampleProvider()
	Println(provider.Name())
	// Output: process
}

func ExampleProcessProvider_BasePath() {
	provider := exampleProvider()
	Println(provider.BasePath())
	// Output: /api/process
}

func ExampleProcessProvider_Register() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	provider := exampleProvider()
	provider.Register(router)
	Println(len(router.Routes()) > 0)
	// Output: true
}

func ExampleProcessProvider_Element() {
	provider := exampleProvider()
	element := provider.Element()
	Println(element.Tag)
	// Output: core-process-panel
}

func ExampleProcessProvider_Channels() {
	provider := exampleProvider()
	Println(Contains(Join(",", provider.Channels()...), "process.started"))
	// Output: true
}

func ExampleProcessProvider_RegisterRoutes() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	provider := exampleProvider()
	provider.RegisterRoutes(router.Group(provider.BasePath()))
	Println(len(router.Routes()) > 0)
	// Output: true
}

func ExampleProcessProvider_Describe() {
	provider := exampleProvider()
	Println(len(provider.Describe()) > 0)
	// Output: true
}

func ExamplePIDAlive() {
	Println(processapi.PIDAlive(Getpid()))
	// Output: true
}
