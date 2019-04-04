package cmd

import (
	"fmt"
	"github.com/bmuschko/lets-gopher/template/archive"
	"github.com/bmuschko/lets-gopher/template/config"
	"github.com/bmuschko/lets-gopher/template/environment"
	"github.com/bmuschko/lets-gopher/template/prompt"
	"github.com/bmuschko/lets-gopher/template/storage"
	"github.com/spf13/cobra"
	"io"
	"path"
	"strings"
)

const keyValueSeparator = "="

type projectCreateCmd struct {
	templateName    string
	templateVersion string
	targetDir       string
	params          []string
	out             io.Writer
	home            storage.Home
	archiver        archive.Archiver
	prompter        prompt.Prompter
}

func newCreateCmd(out io.Writer) *cobra.Command {
	create := &projectCreateCmd{out: out}

	cmd := &cobra.Command{
		Use:   "create [args]",
		Short: "create a new project from a template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "the template name", "the template version", "the target project directory"); err != nil {
				return err
			}

			create.templateName = args[0]
			create.templateVersion = args[1]
			create.targetDir = args[2]
			create.home = environment.Settings.Home
			create.archiver = &archive.ZIPArchiver{Processor: &archive.TemplateProcessor{}}
			create.prompter = &prompt.InteractivePrompter{}
			return create.run()
		},
	}

	cmd.PersistentFlags().StringSliceVar(&create.params, "param", []string{}, "parameter defined as key/value pair separated by = character")
	return cmd
}

func (c *projectCreateCmd) run() error {
	f, err := config.LoadTemplatesFile(c.home.TemplatesFile())
	if err != nil {
		return err
	}
	if !f.Has(c.templateName, c.templateVersion) {
		return fmt.Errorf("template with name %q and version %q hasn't been installed", c.templateName, c.templateVersion)
	}

	templateName := c.templateName + "-" + c.templateVersion
	templateZIP := path.Join(c.home.ArchiveDir(), templateName+".zip")

	tb, err := c.archiver.LoadManifestFile(templateZIP)
	if err != nil {
		return err
	}
	m, err := config.LoadManifestData(tb)
	if err != nil {
		return err
	}
	err = config.ValidateManifest(m)
	if err != nil {
		return err
	}
	userDefinedParams, err := mapUserDefinedParams(c.params)
	if err != nil {
		return err
	}
	r, err := requestParameterValues(userDefinedParams, m.Parameters, c.prompter)
	if err != nil {
		return err
	}

	err = c.archiver.Extract(templateZIP, c.targetDir, r)
	if err != nil {
		return nil
	}
	fmt.Fprintf(c.out, "created project at %q\n", c.targetDir)
	return err
}

func mapUserDefinedParams(params []string) (map[string]string, error) {
	userDefinedParams := make(map[string]string)
	for _, p := range params {
		if !strings.Contains(p, keyValueSeparator) {
			return nil, fmt.Errorf("user-defined parameter %q does not separate key and value by %s character", p, keyValueSeparator)
		}
		s := strings.Split(p, keyValueSeparator)
		userDefinedParams[s[0]] = s[1]
	}
	return userDefinedParams, nil
}

func requestParameterValues(userDefinedParams map[string]string, manifestParams []*config.Parameter, prompter prompt.Prompter) (map[string]interface{}, error) {
	replacements := make(map[string]interface{})

	for _, p := range manifestParams {
		if value, exist := userDefinedParams[p.Name]; exist {
			if p.Enum != nil && !contains(p.Enum, value) {
				return nil, fmt.Errorf("provided value '%s' is not defined in enum [%s]",
					value, strings.Join(p.Enum, ", "))
			}
			replacements[p.Name] = value
			continue
		}

		prompter.Prompt(p, replacements)
	}

	return replacements, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
