package app

// App is an application.
type App struct {
	// Name of the application
	Name string `json:"Name"`

	// Targets is a list of targets that the application is proxying to.
	Targets *Targets `json:"Targets"`

	// Ports is a list of ports that the application is listening on.
	Ports []int `json:"Ports"`
}

// Apps is a list of applications.
type Apps struct {
	// Apps is a list of applications.
	Apps []App `json:"Apps"`
}
