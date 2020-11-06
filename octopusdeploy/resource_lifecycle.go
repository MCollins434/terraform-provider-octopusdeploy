package octopusdeploy

import (
	"context"

	"github.com/OctopusDeploy/go-octopusdeploy/octopusdeploy"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceLifecycle() *schema.Resource {
	resourceLifecycleImporter := &schema.ResourceImporter{
		StateContext: schema.ImportStatePassthroughContext,
	}
	resourceLifecycleSchema := map[string]*schema.Schema{
		constDescription: &schema.Schema{
			Optional: true,
			Type:     schema.TypeString,
		},
		constName: &schema.Schema{
			Required: true,
			Type:     schema.TypeString,
		},
		constPhase:                   getPhasesSchema(),
		constReleaseRetentionPolicy:  getRetentionPeriodSchema(),
		constTentacleRetentionPolicy: getRetentionPeriodSchema(),
	}

	return &schema.Resource{
		CreateContext: resourceLifecycleCreate,
		DeleteContext: resourceLifecycleDelete,
		Importer:      resourceLifecycleImporter,
		ReadContext:   resourceLifecycleRead,
		Schema:        resourceLifecycleSchema,
		UpdateContext: resourceLifecycleUpdate,
	}
}

func getRetentionPeriodSchema() *schema.Schema {
	return &schema.Schema{
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				constQuantityToKeep: {
					Default:     30,
					Description: "The number of days/releases to keep. If 0 all are kept.",
					Optional:    true,
					Type:        schema.TypeInt,
				},
				constShouldKeepForever: {
					Default:  false,
					Optional: true,
					Type:     schema.TypeBool,
				},
				constUnit: {
					Default:     octopusdeploy.RetentionUnitDays,
					Description: "The unit of quantity_to_keep.",
					Optional:    true,
					Type:        schema.TypeString,
					ValidateDiagFunc: validateDiagFunc(validation.StringInSlice([]string{
						octopusdeploy.RetentionUnitDays,
						octopusdeploy.RetentionUnitItems,
					}, false)),
				},
			},
		},
		Optional: true,
		Type:     schema.TypeList,
	}
}

func getPhasesSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				constAutomaticDeploymentTargets: {
					Description: "Environment IDs in this phase that a release is automatically deployed to when it is eligible for this phase",
					Type:        schema.TypeList,
					Optional:    true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				constID: {
					Computed: true,
					Type:     schema.TypeString,
				},
				constName: {
					Type:     schema.TypeString,
					Required: true,
				},
				constMinimumEnvironmentsBeforePromotion: {
					Description: "The number of units required before a release can enter the next phase. If 0, all environments are required.",
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     0,
				},
				constIsOptionalPhase: {
					Description: "If false a release must be deployed to this phase before it can be deployed to the next phase.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
				},
				constOptionalDeploymentTargets: {
					Description: "Environment IDs in this phase that a release can be deployed to, but is not automatically deployed to",
					Type:        schema.TypeList,
					Optional:    true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				constReleaseRetentionPolicy:  getRetentionPeriodSchema(),
				constTentacleRetentionPolicy: getRetentionPeriodSchema(),
			},
		},
	}
}

func resourceLifecycleCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	lifecycle := buildLifecycleResource(d)

	client := m.(*octopusdeploy.Client)
	createdLifecycle, err := client.Lifecycles.Add(lifecycle)
	if err != nil {
		return diag.FromErr(err)
	}

	flattenLifecycle(ctx, d, createdLifecycle)
	return nil
}

func buildLifecycleResource(d *schema.ResourceData) *octopusdeploy.Lifecycle {
	var name string
	if v, ok := d.GetOk(constName); ok {
		name = v.(string)
	}

	lifecycle := octopusdeploy.NewLifecycle(name)

	if v, ok := d.GetOk(constDescription); ok {
		lifecycle.Description = v.(string)
	}

	releaseRetentionPolicy := getRetentionPeriod(d, constReleaseRetentionPolicy)
	if releaseRetentionPolicy != nil {
		lifecycle.ReleaseRetentionPolicy = *releaseRetentionPolicy
	}

	tentacleRetentionPolicy := getRetentionPeriod(d, constTentacleRetentionPolicy)
	if tentacleRetentionPolicy != nil {
		lifecycle.TentacleRetentionPolicy = *tentacleRetentionPolicy
	}

	if attr, ok := d.GetOk(constPhase); ok {
		tfPhases := attr.([]interface{})

		for _, tfPhase := range tfPhases {
			phase := buildPhaseResource(tfPhase.(map[string]interface{}))
			lifecycle.Phases = append(lifecycle.Phases, phase)
		}
	}

	return lifecycle
}

func getRetentionPeriod(d *schema.ResourceData, key string) *octopusdeploy.RetentionPeriod {
	v, ok := d.GetOk(key)
	if ok {
		retentionPeriod := v.([]interface{})
		if len(retentionPeriod) == 1 {
			tfRetentionItem := retentionPeriod[0].(map[string]interface{})
			retention := octopusdeploy.RetentionPeriod{
				Unit:           tfRetentionItem[constUnit].(string),
				QuantityToKeep: int32(tfRetentionItem[constQuantityToKeep].(int)),
			}
			return &retention
		}
	}

	return nil
}

func buildPhaseResource(tfPhase map[string]interface{}) octopusdeploy.Phase {
	phase := octopusdeploy.Phase{
		Name:                               tfPhase[constName].(string),
		MinimumEnvironmentsBeforePromotion: int32(tfPhase[constMinimumEnvironmentsBeforePromotion].(int)),
		IsOptionalPhase:                    tfPhase[constIsOptionalPhase].(bool),
		AutomaticDeploymentTargets:         getSliceFromTerraformTypeList(tfPhase[constAutomaticDeploymentTargets]),
		OptionalDeploymentTargets:          getSliceFromTerraformTypeList(tfPhase[constOptionalDeploymentTargets]),
	}

	if phase.AutomaticDeploymentTargets == nil {
		phase.AutomaticDeploymentTargets = []string{}
	}
	if phase.OptionalDeploymentTargets == nil {
		phase.OptionalDeploymentTargets = []string{}
	}

	return phase
}

func resourceLifecycleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*octopusdeploy.Client)
	lifecycle, err := client.Lifecycles.GetByID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	flattenLifecycle(ctx, d, lifecycle)
	return nil
}

func resourceLifecycleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	lifecycle := buildLifecycleResource(d)
	lifecycle.ID = d.Id()

	client := m.(*octopusdeploy.Client)
	updatedLifecycle, err := client.Lifecycles.Update(lifecycle)
	if err != nil {
		return diag.FromErr(err)
	}

	flattenLifecycle(ctx, d, updatedLifecycle)
	return nil
}

func resourceLifecycleDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*octopusdeploy.Client)
	err := client.Lifecycles.DeleteByID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(constEmptyString)
	return nil
}

func flattenPhase(p octopusdeploy.Phase) []interface{} {
	phase := make(map[string]interface{})
	phase[constAutomaticDeploymentTargets] = p.AutomaticDeploymentTargets
	phase[constID] = p.ID
	phase[constIsOptionalPhase] = p.IsOptionalPhase
	phase[constMinimumEnvironmentsBeforePromotion] = p.MinimumEnvironmentsBeforePromotion
	phase[constName] = p.Name
	phase[constOptionalDeploymentTargets] = p.OptionalDeploymentTargets
	phase[constReleaseRetentionPolicy] = p.ReleaseRetentionPolicy
	phase[constTentacleRetentionPolicy] = p.TentacleRetentionPolicy
	return []interface{}{phase}
}

func flattenRetentionPeriod(r octopusdeploy.RetentionPeriod) []interface{} {
	retentionPeriod := make(map[string]interface{})
	retentionPeriod[constUnit] = r.Unit
	retentionPeriod[constQuantityToKeep] = r.QuantityToKeep
	retentionPeriod[constShouldKeepForever] = r.ShouldKeepForever
	return []interface{}{retentionPeriod}
}

func flattenLifecycle(ctx context.Context, d *schema.ResourceData, lifecycle *octopusdeploy.Lifecycle) {
	d.Set(constDescription, lifecycle.Description)
	d.Set(constName, lifecycle.Name)

	for _, phase := range lifecycle.Phases {
		d.Set(constPhase, flattenPhase(phase))
	}

	d.Set(constReleaseRetentionPolicy, flattenRetentionPeriod(lifecycle.ReleaseRetentionPolicy))
	d.Set(constTentacleRetentionPolicy, flattenRetentionPeriod(lifecycle.TentacleRetentionPolicy))

	d.SetId(lifecycle.GetID())
}
