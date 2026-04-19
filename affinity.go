package affinity

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

const (
	labelColocate = "swarmex.affinity.colocate" // place on same node as target service
	labelAvoid    = "swarmex.affinity.avoid"     // place on different node than target
	labelSpread   = "swarmex.affinity.spread"    // spread across node label values
)

// Controller manages service placement based on affinity labels.
type Controller struct {
	client *client.Client
	logger *slog.Logger
}

func New(cli *client.Client, logger *slog.Logger) *Controller {
	return &Controller{client: cli, logger: logger}
}

func (c *Controller) HandleEvent(ctx context.Context, event events.Message) {
	if event.Type != events.ServiceEventType {
		return
	}
	if event.Action != "create" && event.Action != "update" {
		return
	}
	c.reconcile(ctx, event.Actor.ID)
}

func (c *Controller) reconcile(ctx context.Context, serviceID string) {
	svc, _, err := c.client.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return
	}

	labels := svc.Spec.Labels
	if labels[labelColocate] == "" && labels[labelAvoid] == "" && labels[labelSpread] == "" {
		return
	}

	var constraints []string
	changed := false

	// Colocate: find which node the target runs on, constrain to that node
	if target := labels[labelColocate]; target != "" {
		if nodeID := c.findServiceNode(ctx, target); nodeID != "" {
			constraint := fmt.Sprintf("node.id==%s", nodeID)
			if !containsConstraint(getConstraints(svc.Spec.TaskTemplate.Placement), constraint) {
				constraints = append(constraints, constraint)
				c.logger.Info("affinity colocate", "service", svc.Spec.Name, "with", target, "node", nodeID)
			}
		}
	}

	// Avoid: find which node the target runs on, constrain to NOT that node
	if target := labels[labelAvoid]; target != "" {
		if nodeID := c.findServiceNode(ctx, target); nodeID != "" {
			constraint := fmt.Sprintf("node.id!=%s", nodeID)
			if !containsConstraint(getConstraints(svc.Spec.TaskTemplate.Placement), constraint) {
				constraints = append(constraints, constraint)
				c.logger.Info("affinity avoid", "service", svc.Spec.Name, "avoiding", target, "node", nodeID)
			}
		}
	}

	// Spread: use placement preferences on a node label
	if spreadLabel := labels[labelSpread]; spreadLabel != "" {
		descriptor := fmt.Sprintf("node.labels.%s", spreadLabel)
		prefs := getPreferences(svc.Spec.TaskTemplate.Placement)
		hasSpread := false
		for _, p := range prefs {
			if p.Spread != nil && p.Spread.SpreadDescriptor == descriptor {
				hasSpread = true
				break
			}
		}
		if !hasSpread {
			if svc.Spec.TaskTemplate.Placement == nil {
				svc.Spec.TaskTemplate.Placement = &swarm.Placement{}
			}
			svc.Spec.TaskTemplate.Placement.Preferences = []swarm.PlacementPreference{
				{Spread: &swarm.SpreadOver{SpreadDescriptor: descriptor}},
			}
			changed = true
			c.logger.Info("affinity spread", "service", svc.Spec.Name, "label", spreadLabel)
		}
	}

	if len(constraints) > 0 {
		if svc.Spec.TaskTemplate.Placement == nil {
			svc.Spec.TaskTemplate.Placement = &swarm.Placement{}
		}
		svc.Spec.TaskTemplate.Placement.Constraints = append(svc.Spec.TaskTemplate.Placement.Constraints, constraints...)
		changed = true
	}

	if !changed {
		return
	}

	_, err = c.client.ServiceUpdate(ctx, serviceID, svc.Version, svc.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		c.logger.Error("affinity update failed", "service", svc.Spec.Name, "error", err)
	}
}

func (c *Controller) findServiceNode(ctx context.Context, serviceName string) string {
	// Find service by name
	services, err := c.client.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("name", serviceName)),
	})
	if err != nil || len(services) == 0 {
		return ""
	}
	// Find a running task for this service
	tasks, err := c.client.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(
			filters.Arg("service", services[0].ID),
			filters.Arg("desired-state", "running"),
		),
	})
	if err != nil || len(tasks) == 0 {
		return ""
	}
	return tasks[0].NodeID
}

func containsConstraint(constraints []string, target string) bool {
	for _, c := range constraints {
		if c == target {
			return true
		}
	}
	return false
}

func getConstraints(p *swarm.Placement) []string {
	if p == nil {
		return nil
	}
	return p.Constraints
}

func getPreferences(p *swarm.Placement) []swarm.PlacementPreference {
	if p == nil {
		return nil
	}
	return p.Preferences
}
