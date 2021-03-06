package provisioning

import (
	"fmt"
	"time"

	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/runtimeoverrides"

	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/broker"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/storage"

	"github.com/sirupsen/logrus"
)

type RuntimeOverridesAppender interface {
	Append(input runtimeoverrides.InputAppender, planName, kymaVersion string) error
}

//go:generate mockery --name=RuntimeVersionConfiguratorForProvisioning --output=automock --outpkg=automock --case=underscore
type RuntimeVersionConfiguratorForProvisioning interface {
	ForProvisioning(op internal.ProvisioningOperation) (*internal.RuntimeVersionData, error)
}

type OverridesFromSecretsAndConfigStep struct {
	operationManager       *process.ProvisionOperationManager
	runtimeOverrides       RuntimeOverridesAppender
	runtimeVerConfigurator RuntimeVersionConfiguratorForProvisioning
}

func NewOverridesFromSecretsAndConfigStep(os storage.Operations, runtimeOverrides RuntimeOverridesAppender,
	rvc RuntimeVersionConfiguratorForProvisioning) *OverridesFromSecretsAndConfigStep {
	return &OverridesFromSecretsAndConfigStep{
		operationManager:       process.NewProvisionOperationManager(os),
		runtimeOverrides:       runtimeOverrides,
		runtimeVerConfigurator: rvc,
	}
}

func (s *OverridesFromSecretsAndConfigStep) Name() string {
	return "Overrides_From_Secrets_And_Config_Step"
}

func (s *OverridesFromSecretsAndConfigStep) Run(operation internal.ProvisioningOperation, log logrus.FieldLogger) (internal.ProvisioningOperation, time.Duration, error) {
	planName, exists := broker.PlanNamesMapping[operation.ProvisioningParameters.PlanID]
	if !exists {
		log.Errorf("cannot map planID '%s' to planName", operation.ProvisioningParameters.PlanID)
		return s.operationManager.OperationFailed(operation, "invalid operation provisioning parameters", log)
	}

	version, err := s.getRuntimeVersion(operation)
	if err != nil {
		errMsg := fmt.Sprintf("error while getting the runtime version for operation %s", operation.ID)
		log.Error(errMsg)
		return s.operationManager.RetryOperation(operation, errMsg, 10*time.Second, 30*time.Minute, log)
	}

	if err := s.runtimeOverrides.Append(operation.InputCreator, planName, version.Version); err != nil {
		errMsg := fmt.Sprintf("error when appending overrides for operation %s: %s", operation.ID, err.Error())
		log.Error(errMsg)
		return s.operationManager.RetryOperation(operation, errMsg, 10*time.Second, 30*time.Minute, log)
	}

	return operation, 0, nil
}

func (s *OverridesFromSecretsAndConfigStep) getRuntimeVersion(op internal.ProvisioningOperation) (*internal.RuntimeVersionData, error) {
	// for some previously stored operations the RuntimeVersion property may not be initialized
	if op.RuntimeVersion.Version != "" {
		return &op.RuntimeVersion, nil
	}

	// if so, we manually compute the correct version using the same algorithm as when preparing
	// the provisioning operation. The following code can be removed after all operations will use
	// new approach for setting up runtime version in operation struct
	return s.runtimeVerConfigurator.ForProvisioning(op)
}
