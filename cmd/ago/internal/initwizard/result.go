package initwizard

type Result struct {
	ProjectIdent      string
	PrimaryRegion     string
	SecondaryRegions  []string
	ManagementProfile string
	InitialDeployer   string
}
