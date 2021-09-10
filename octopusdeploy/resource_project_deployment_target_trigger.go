package octopusdeploy

import (
	"context"
	"fmt"
	"log"

	"github.com/OctopusDeploy/go-octopusdeploy/octopusdeploy"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProjectDeploymentTargetTrigger() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceProjectDeploymentTargetTriggerCreate,
		DeleteContext: resourceProjectDeploymentTargetTriggerDelete,
		Importer:      getImporter(),
		ReadContext:   resourceProjectDeploymentTargetTriggerRead,
		Schema:        getProjectDeploymentTargetTriggerSchema(),
		UpdateContext: resourceProjectDeploymentTargetTriggerUpdate,
	}
}

func buildProjectDeploymentTargetTriggerResource(d *schema.ResourceData) (*octopusdeploy.ProjectTrigger, error) {
	name := d.Get("name").(string)
	projectID := d.Get("project_id").(string)
	shouldRedeploy := d.Get("should_redeploy").(bool)

	deploymentTargetTrigger := octopusdeploy.NewProjectDeploymentTargetTrigger(name, projectID, shouldRedeploy, nil, nil, nil)

	if attr, ok := d.GetOk("event_groups"); ok {
		eventGroups := getSliceFromTerraformTypeList(attr)

		// need to validate here "ValidateFunc is not yet supported on lists or sets."
		validValues := []string{
			"Machine",
			"MachineCritical",
			"MachineAvailableForDeployment",
			"MachineUnavailableForDeployment",
			"MachineHealthChanged",
		}

		if invalidValue, ok := validateAllSliceItemsInSlice(eventGroups, validValues); !ok {
			return nil, fmt.Errorf("Invalid value for event_groups. %s not in %v", invalidValue, validValues)
		}

		deploymentTargetTrigger.AddEventGroups(eventGroups)
	}

	if attr, ok := d.GetOk("event_categories"); ok {
		eventCategories := getSliceFromTerraformTypeList(attr)

		// need to validate here "ValidateFunc is not yet supported on lists or sets."
		validValues := []string{
			"MachineCleanupFailed",
			"MachineAdded",
			"MachineDeploymentRelatedPropertyWasUpdated",
			"MachineDisabled",
			"MachineEnabled",
			"MachineHealthy",
			"MachineUnavailable",
			"MachineUnhealthy",
			"MachineHasWarnings",
		}

		if invalidValue, ok := validateAllSliceItemsInSlice(eventCategories, validValues); !ok {
			return nil, fmt.Errorf("Invalid value for event_categories. %s not in %v", invalidValue, validValues)
		}

		deploymentTargetTrigger.AddEventCategories(eventCategories)
	}

	if attr, ok := d.GetOk("roles"); ok {
		deploymentTargetTrigger.Filter.Roles = getSliceFromTerraformTypeList(attr)
	}

	if attr, ok := d.GetOk("environment_ids"); ok {
		deploymentTargetTrigger.Filter.EnvironmentIDs = getSliceFromTerraformTypeList(attr)
	}

	return deploymentTargetTrigger, nil
}

func resourceProjectDeploymentTargetTriggerCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	projectTrigger, err := buildProjectDeploymentTargetTriggerResource(d)
	if err != nil {
		return diag.FromErr(err)
	}

	client := m.(*octopusdeploy.Client)
	resource, err := client.ProjectTriggers.Add(projectTrigger)
	if err != nil {
		return diag.FromErr(err)
	}

	if isEmpty(resource.GetID()) {
		log.Println("ID is nil")
	} else {
		d.SetId(resource.GetID())
	}

	return nil
}

func resourceProjectDeploymentTargetTriggerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	id := d.Id()

	client := m.(*octopusdeploy.Client)
	resource, err := client.ProjectTriggers.GetByID(id)
	if err != nil {
		return diag.FromErr(err)
	}
	if resource == nil {
		d.SetId("")
		return nil
	}

	logResource("project_trigger", m)

	d.Set("environment_ids", resource.Filter.EnvironmentIDs)
	d.Set("event_groups", resource.Filter.EventGroups)
	d.Set("event_categories", resource.Filter.EventCategories)
	d.Set("name", resource.Name)
	d.Set("roles", resource.Filter.Roles)
	d.Set("should_redeploy", resource.Action.ShouldRedeployWhenMachineHasBeenDeployedTo)

	return nil
}

func resourceProjectDeploymentTargetTriggerUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	projectTrigger, err := buildProjectDeploymentTargetTriggerResource(d)
	if err != nil {
		return diag.FromErr(err)
	}
	projectTrigger.ID = d.Id() // set ID so Octopus API knows which project trigger to update

	client := m.(*octopusdeploy.Client)
	resource, err := client.ProjectTriggers.Update(*projectTrigger)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resource.GetID())

	return nil
}

func resourceProjectDeploymentTargetTriggerDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*octopusdeploy.Client)
	err := client.ProjectTriggers.DeleteByID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
