package octopusdeploy

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/terraform-provider-octopusdeploy/internal/errors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDeploymentProcess() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDeploymentProcessCreate,
		DeleteContext: resourceDeploymentProcessDelete,
		Description:   "This resource manages deployment processes in Octopus Deploy.",
		Importer:      getImporter(),
		ReadContext:   resourceDeploymentProcessRead,
		Schema:        getDeploymentProcessSchema(),
		UpdateContext: resourceDeploymentProcessUpdate,
	}
}

func getDeploymentProcessSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"id": getIDSchema(),
		"branch": {
			Computed:    true,
			Description: "The branch name associated with this deployment process (i.e. `main`). This value is optional and only applies to associated projects that are stored in version control.",
			Optional:    true,
			Type:        schema.TypeString,
		},
		"last_snapshot_id": {
			Optional: true,
			Type:     schema.TypeString,
		},
		"project_id": {
			Description: "The project ID associated with this deployment process.",
			Required:    true,
			Type:        schema.TypeString,
		},
		"space_id": getSpaceIDSchema(),
		"step":     getDeploymentStepSchema(),
		"version": {
			Computed:    true,
			Description: "The version number of this deployment process.",
			Optional:    true,
			Type:        schema.TypeInt,
		},
	}
}

func resourceDeploymentProcessCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*client.Client)
	deploymentProcess := expandDeploymentProcess(ctx, d, client)

	log.Printf("[INFO] creating deployment process: %#v", deploymentProcess)

	project, err := client.Projects.GetByID(deploymentProcess.ProjectID)
	if err != nil {
		return diag.FromErr(err)
	}

	var current *deployments.DeploymentProcess
	if project.PersistenceSettings != nil && project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		current, err = client.DeploymentProcesses.Get(project, deploymentProcess.Branch)
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		current, err = client.DeploymentProcesses.GetByID(project.DeploymentProcessID)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	deploymentProcess.ID = current.ID
	deploymentProcess.Links = current.Links
	deploymentProcess.Version = current.Version

	createdDeploymentProcess, err := client.DeploymentProcesses.Update(deploymentProcess)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := setDeploymentProcess(ctx, d, createdDeploymentProcess); err != nil {
		return diag.FromErr(err)
	}

	id := createdDeploymentProcess.GetID()
	if project.PersistenceSettings != nil && project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		id = "deploymentprocess-" + createdDeploymentProcess.ProjectID + "-" + deploymentProcess.Branch
	}

	d.SetId(id)

	log.Printf("[INFO] deployment process created (%s)", d.Id())
	return nil
}

func resourceDeploymentProcessDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	log.Printf("[INFO] deleting deployment process (%s)", d.Id())

	client := m.(*client.Client)
	current, err := client.DeploymentProcesses.GetByID(d.Id())
	if err == nil {
		deploymentProcess := &deployments.DeploymentProcess{
			Version: current.Version,
		}
		deploymentProcess.Links = current.Links
		deploymentProcess.ID = d.Id()

		_, err = client.DeploymentProcesses.Update(deploymentProcess)
		if err != nil {
			return diag.FromErr(err)
		}

		d.SetId("")
		log.Printf("[INFO] deployment process deleted")
		return nil
	}

	r, _ := regexp.Compile(`Projects-\d+`)
	projectID := r.FindString(d.Id())

	project, err := client.Projects.GetByID(projectID)
	if err != nil {
		return diag.FromErr(err)
	}

	gitRef := getGitRef(d)
	current, err = client.DeploymentProcesses.Get(project, gitRef)
	if err != nil {
		return diag.FromErr(err)
	}

	deploymentProcess := &deployments.DeploymentProcess{
		Version: current.Version,
	}
	deploymentProcess.Links = current.Links
	deploymentProcess.ID = d.Id()

	_, err = client.DeploymentProcesses.Update(deploymentProcess)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	log.Printf("[INFO] deployment process deleted")
	return nil
}

func resourceDeploymentProcessRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	log.Printf("[INFO] reading deployment process (%s)", d.Id())

	client := m.(*client.Client)
	deploymentProcess, err := client.DeploymentProcesses.GetByID(d.Id())
	if err == nil {
		if err := setDeploymentProcess(ctx, d, deploymentProcess); err != nil {
			return diag.FromErr(err)
		}

		log.Printf("[INFO] deployment process read (%s)", d.Id())
		return nil
	}

	r, _ := regexp.Compile(`Projects-\d+`)
	projectID := r.FindString(d.Id())

	project, err := client.Projects.GetByID(projectID)
	if err != nil {
		return errors.ProcessApiError(ctx, d, err, "project")
	}

	gitRef := getGitRef(d)
	deploymentProcess, err = client.DeploymentProcesses.Get(project, gitRef)
	if err == nil {
		if err := setDeploymentProcess(ctx, d, deploymentProcess); err != nil {
			return diag.FromErr(err)
		}

		log.Printf("[INFO] deployment process read (%s)", d.Id())
		return nil
	}

	return errors.DeleteFromState(ctx, d, "deployment process")
}

func resourceDeploymentProcessUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	log.Printf("[INFO] updating deployment process (%s)", d.Id())

	client := m.(*client.Client)
	deploymentProcess := expandDeploymentProcess(ctx, d, client)
	current, err := client.DeploymentProcesses.GetByID(d.Id())
	if err != nil {
		r, _ := regexp.Compile(`Projects-\d+`)
		projectID := r.FindString(d.Id())

		project, err := client.Projects.GetByID(projectID)
		if err != nil {
			return diag.FromErr(err)
		}

		gitRef := getGitRef(d)
		if deploymentProcess.Branch != gitRef && gitRef != "" { //if gitRef is empty, its likely this is a conversion of an existing deployment process
			return diag.Errorf("you cannot change a deployment processes branch. instead create a new resource with the new branch and, if required, destroy the previous one")
		}

		if project.PersistenceSettings != nil && project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
			deploymentProcess.ID = "deploymentprocess-" + projectID + "-" + deploymentProcess.Branch
			d.SetId(deploymentProcess.ID)
		}

		current, err = client.DeploymentProcesses.Get(project, deploymentProcess.Branch)

		if err != nil {
			return diag.FromErr(err)
		}
	}

	deploymentProcess.Links = current.Links
	deploymentProcess.Version = current.Version

	updatedDeploymentProcess, err := client.DeploymentProcesses.Update(deploymentProcess)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := setDeploymentProcess(ctx, d, updatedDeploymentProcess); err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] deployment process updated (%s)", d.Id())
	return nil
}

func getGitRef(d *schema.ResourceData) string {
	r, _ := regexp.Compile(`\d+-\w+`)
	parts := strings.SplitAfter(r.FindString(d.Id()), "-")
	if len(parts) > 2 {
		return parts[1]
	}
	return ""
}
