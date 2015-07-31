package packs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloud66/starter/common"
)

type AnalyzerBase struct {
	PackElement

	RootDir      string
	Environment  string
	ShouldPrompt bool

	Messages common.Lister
}

func (a *AnalyzerBase) ProjectMetadata() (string, string, string, error) {
	gitURL := common.RemoteGitUrl()
	gitBranch := common.LocalGitBranch()
	buildRoot, err := common.PathRelativeToGitRoot(a.RootDir)
	if err != nil {
		return "", "", "", err
	}

	return gitURL, gitBranch, buildRoot, nil
}

func (a *AnalyzerBase) ConfirmDatabases(foundDbs *common.Lister) *common.Lister {
	var dbs common.Lister
	for _, db := range foundDbs.Items {
		if !a.ShouldPrompt {
			fmt.Println(common.MsgL2, fmt.Sprintf("----> Found %s", db), common.MsgReset)
		}
		if common.AskYesOrNo(common.MsgL2, fmt.Sprintf("----> Found %s, confirm?", db), true, a.ShouldPrompt) {
			dbs.Add(db)
		}
	}

	var message string
	var defaultValue bool
	if len(foundDbs.Items) > 0 {
		message = "Add any other databases?"
		defaultValue = false
	} else {
		message = "No databases found. Add manually?"
		defaultValue = true
	}

	if common.AskYesOrNo(common.MsgL1, message, defaultValue, a.ShouldPrompt) && a.ShouldPrompt {
		fmt.Println(common.MsgL1, fmt.Sprintf("  See http://help.cloud66.com/building-your-stack/docker-service-configuration#database-configs for complete list of possible values"), common.MsgReset)
		fmt.Println(common.MsgL1, fmt.Sprintf("  Example: 'mysql elasticsearch' "), common.MsgReset)
		fmt.Print(" > ")

		reader := bufio.NewReader(os.Stdin)
		otherDbs, err := reader.ReadString('\n')
		if err == nil {
			dbs.Add(strings.Fields(otherDbs)...)
		}
	}
	return &dbs
}

func (a *AnalyzerBase) ConfirmVersion(found bool, version string, defaultVersion string) string {
	message := fmt.Sprintf("Found %s version %s, confirm?", a.GetPack().Name(), version)
	if found && common.AskYesOrNo(common.MsgL1, message, true, a.ShouldPrompt) {
		return version
	}
	return common.AskUserWithDefault(fmt.Sprintf("Enter %s version:", a.GetPack().Name()), defaultVersion, a.ShouldPrompt)
}

func (b *AnalyzerBase) AnalyzeServices(a Analyzer, envVars []*common.EnvVar, gitBranch string, gitURL string, buildRoot string) ([]*common.Service, error) {
	services, err := b.analyzeProcfile()
	if err != nil {
		fmt.Printf("%s Failed to parse Procfile due to %s\n", common.MsgError, err.Error())
		return nil, err
	}

	err = a.FillServices(&services)
	if err != nil {
		return nil, err
	}
	b.refineServices(&services)
	b.inheritProjectContext(&services, envVars, gitBranch, gitURL, buildRoot)
	return services, nil
}

func (a *AnalyzerBase) analyzeProcfile() ([]*common.Service, error) {
	services := []*common.Service{}
	procfilePath := filepath.Join(a.RootDir, "Procfile")
	if !common.FileExists(procfilePath) {
		return services, nil
	}

	fmt.Println(common.MsgL1, "Parsing Procfile")
	procs, err := common.ParseProcfile(procfilePath)
	if err != nil {
		return nil, err
	}

	for _, proc := range procs {
		fmt.Printf("%s ----> Found Procfile item %s\n", common.MsgL2, proc.Name)
		services = append(services, &common.Service{Name: proc.Name, Command: proc.Command})
	}
	return services, nil
}

func (a *AnalyzerBase) GetOrCreateWebService(services *[]*common.Service) *common.Service {
	var service *common.Service
	for _, s := range *services {
		if s.Name == "web" || s.Name == "custom_web" {
			service = s
			break
		}
	}
	if service == nil {
		service = &common.Service{Name: "web"}
		*services = append(*services, service)
	}
	return service
}

func (a *AnalyzerBase) refineServices(services *[]*common.Service) {
	var err error
	for _, service := range *services {
		if service.Command, err = common.ParseEnvironmentVariables(service.Command); err != nil {
			fmt.Printf("%s Failed to replace environment variable placeholder due to %s\n", common.MsgError, err.Error())
		}

		if service.Command, err = common.ParseUniqueInt(service.Command); err != nil {
			fmt.Printf("%s Failed to replace UNIQUE_INT variable placeholder due to %s\n", common.MsgError, err.Error())
		}
	}
}

func (a *AnalyzerBase) inheritProjectContext(services *[]*common.Service, envVars []*common.EnvVar, gitBranch string, gitURL string, buildRoot string) {
	for _, service := range *services {
		service.EnvVars = envVars
		service.GitBranch = gitBranch
		service.GitRepo = gitURL
		service.BuildRoot = buildRoot
	}
}
