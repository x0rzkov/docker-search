package main

import (
	"fmt"
	"flag"
	"os"
	"os/user"
	"path"
)

func help() {
		doc := `

docker-search: A better way to search the docker registry

docker-search can be used to search for specific information inside 
the docker registry. Want a docker base image with a specific version of 
ruby? Want to know what build flags were used when compiling that ffmpeg
binary? docker-search can help with all of that. docker-search searches 
not only for the name but also peers into the Dockerfile used to build 
the base image, and even traces back through the image's ancestry. 

Examples:

docker-search ffmpeg -filter=ruby:1.9  # Search for base images with ffpmeg with ruby 1.9
docker-search ffmpeg -filter=libavcodec:2.2:src # images with python 2.2 compiled from source 
docker-search ffmpeg -dockerfile # print out full dockerfiles

Flags:

--generate-config   # Create a new configuration file from current defaults
--dockerfile[s]     # Print out full Dockerfile with each results
--filter=str        # Search for the string in the Dockerfile
--string=str:src    # Search for the string with a filter of match

`
	fmt.Println( doc )

}

const DEFAULT_CONFIG_FILE = ".docker-search.toml"

func getHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		fmt.Println( "Error: ", err )
	}
	return usr.HomeDir 
}

func getConfigFilePath() string {
	return path.Join( getHomeDir(), DEFAULT_CONFIG_FILE )
}

func generateDefaultConfiguration() {
	configPath := getConfigFilePath()
	
	// Generate configuration
	if file, err := os.Create( configPath ); nil == err {
		def := `
Host = "http://192.168.59.103:2375"
Endpoint = "/v1.12/images/search"
UpdateCheck = true
`
		file.Write( []byte(def) )
		fmt.Println( "Created new configuration file" )
	} else {
		fmt.Println( "Unable to create new configuration file at: ", configPath, err )
	}
}


func main() {
	
	// var ip  = flag.Int("flagname", 1234, "help message for
	// flagname")
	var genCon = flag.Bool( "generate-config", false, "Generate a new default configuration file" )
	flag.Parse()

	if *genCon {
		generateDefaultConfiguration()
		flag.PrintDefaults()
	} else {
		c := new(Client)
		if c.LoadConfig( getConfigFilePath() ) {
			c.Query( flag.Arg(0) )
			result := c.Filter( flag.Args()[1:] )
			fmt.Println( "Search results: ", result )
		} else {
			fmt.Println( "No configuration file found, use --generate-config" )
			
		}
	}
}
