package initwizard

type Result struct {
	ProjectIdent      string
	PrimaryRegion     string
	SecondaryRegions  []string
	ManagementProfile string
	InitialDeployer   string
	TerraformCloudOrg string
}

func DefaultResult(defaultIdent string) Result {
	return Result{
		ProjectIdent:      defaultIdent,
		PrimaryRegion:     "eu-central-1",
		SecondaryRegions:  []string{"eu-north-1"},
		ManagementProfile: "crewlinker-management-account",
		InitialDeployer:   "Adam",
		TerraformCloudOrg: "basewarp",
	}
}
