package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kyma-project/control-plane/components/kyma-environment-broker/common/hyperscaler"

	"github.com/kyma-project/control-plane/components/provisioner/pkg/gqlschema"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"

	gardenerapi "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerFake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/common/orchestration"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/avs"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/broker"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/event"
	kebOrchestration "github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/orchestration"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process/input"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process/input/automock"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process/upgrade_cluster"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/process/upgrade_kyma"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/provisioner"
	kebRuntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/runtime"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/runtimeoverrides"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/runtimeversion"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/storage"
	"github.com/kyma-project/kyma/components/kyma-operator/pkg/apis/installer/v1alpha1"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	globalAccountLabel     = "account"
	subAccountLabel        = "subaccount"
	runtimeIDAnnotation    = "kcp.provisioner.kyma-project.io/runtime-id"
	defaultNamespace       = "kcp-system"
	kymaVersionsConfigName = "kyma-versions"
	defaultRegion          = "cf-eu10"
)

type OrchestrationSuite struct {
	gardenerNamespace string
	provisionerClient *provisioner.FakeClient
	kymaQueue         *process.Queue
	clusterQueue      *process.Queue
	storage           storage.BrokerStorage
	gardenerClient    *gardenerFake.Clientset

	t *testing.T
}

func NewOrchestrationSuite(t *testing.T, additionalKymaVersions []string) *OrchestrationSuite {
	logs := logrus.New()
	logs.Formatter.(*logrus.TextFormatter).TimestampFormat = "15:04:05.000"

	var cfg Config
	cfg.Ems.Disabled = true
	cfg.Cls.Disabled = true

	optionalComponentsDisablers := kebRuntime.ComponentsDisablers{}
	optComponentsSvc := kebRuntime.NewOptionalComponentsService(optionalComponentsDisablers)

	disabledComponentsProvider := kebRuntime.NewDisabledComponentsProvider()

	componentListProvider := &automock.ComponentListProvider{}
	componentListProvider.On("AllComponents", mock.Anything).Return([]v1alpha1.KymaComponent{}, nil)

	defaultKymaVer := "1.15.1"
	inputFactory, err := input.NewInputBuilderFactory(optComponentsSvc, disabledComponentsProvider, componentListProvider, input.Config{
		MachineImageVersion:         "coreos",
		KubernetesVersion:           "1.18",
		MachineImage:                "253",
		Timeout:                     time.Minute,
		URL:                         "http://localhost",
		DefaultGardenerShootPurpose: "testing",
	}, defaultKymaVer, map[string]string{"cf-eu10": "europe"})
	require.NoError(t, err)

	ctx, _ := context.WithTimeout(context.Background(), 20*time.Minute)
	db := storage.NewMemoryStorage()
	sch := runtime.NewScheme()
	require.NoError(t, coreV1.AddToScheme(sch))
	cli := fake.NewFakeClientWithScheme(sch, fixK8sResources(defaultKymaVer, additionalKymaVersions)...)

	gardenerClient := gardenerFake.NewSimpleClientset()
	provisionerClient := provisioner.NewFakeClient()
	const gardenerProject = "testing"
	gardenerNamespace := fmt.Sprintf("garden-%s", gardenerProject)

	eventBroker := event.NewPubSub(logs)

	runtimeOverrides := runtimeoverrides.NewRuntimeOverrides(ctx, cli)

	runtimeVerConfigurator := runtimeversion.NewRuntimeVersionConfigurator(defaultKymaVer, runtimeversion.NewAccountVersionMapping(ctx, cli, defaultNamespace, kymaVersionsConfigName, logs))

	avsClient, _ := avs.NewClient(ctx, avs.Config{}, logs)
	avsDel := avs.NewDelegator(avsClient, avs.Config{}, db.Operations())
	upgradeEvaluationManager := avs.NewEvaluationManager(avsDel, avs.Config{})
	runtimeLister := kebOrchestration.NewRuntimeLister(db.Instances(), db.Operations(), kebRuntime.NewConverter(defaultRegion), logs)
	runtimeResolver := orchestration.NewGardenerRuntimeResolver(gardenerClient.CoreV1beta1(), gardenerNamespace, runtimeLister, logs)

	kymaQueue := NewKymaOrchestrationProcessingQueue(ctx, db, runtimeOverrides, provisionerClient, eventBroker, inputFactory, &upgrade_kyma.TimeSchedule{
		Retry:              10 * time.Millisecond,
		StatusCheck:        100 * time.Millisecond,
		UpgradeKymaTimeout: 4 * time.Second,
	}, 250*time.Millisecond, runtimeVerConfigurator, runtimeResolver, upgradeEvaluationManager,
		&cfg, hyperscaler.NewAccountProvider(nil, nil, nil), nil, nil, logs)

	clusterQueue := NewClusterOrchestrationProcessingQueue(ctx, db, provisionerClient, eventBroker, inputFactory, &upgrade_cluster.TimeSchedule{
		Retry:                 10 * time.Millisecond,
		StatusCheck:           100 * time.Millisecond,
		UpgradeClusterTimeout: 4 * time.Second,
	}, 250*time.Millisecond, runtimeResolver, upgradeEvaluationManager, logs)

	return &OrchestrationSuite{
		gardenerNamespace: gardenerNamespace,
		provisionerClient: provisionerClient,
		kymaQueue:         kymaQueue,
		clusterQueue:      clusterQueue,
		storage:           db,
		gardenerClient:    gardenerClient,

		t: t,
	}
}

type RuntimeOptions struct {
	GlobalAccountID string
	SubAccountID    string
	PlatformRegion  string
	Region          string
}

func (o *RuntimeOptions) ProvideRegion() *string {
	if o.Region != "" {
		return &o.Region
	} else {
		r := "westeurope"
		return &r
	}
}

func (o *RuntimeOptions) ProvideGlobalAccountID() string {
	if o.GlobalAccountID != "" {
		return o.GlobalAccountID
	} else {
		return uuid.New()
	}
}

func (o *RuntimeOptions) ProvideSubAccountID() string {
	if o.SubAccountID != "" {
		return o.SubAccountID
	} else {
		return uuid.New()
	}
}

func (o *RuntimeOptions) ProvidePlatformRegion() string {
	if o.PlatformRegion != "" {
		return o.PlatformRegion
	} else {
		return "cf-eu10"
	}
}

func (s *OrchestrationSuite) CreateProvisionedRuntime(options RuntimeOptions) string {
	planID := broker.AzurePlanID
	planName := broker.AzurePlanName
	runtimeID := uuid.New()
	globalAccountID := options.ProvideGlobalAccountID()
	subAccountID := options.ProvideSubAccountID()
	instanceID := uuid.New()
	provisioningParameters := internal.ProvisioningParameters{
		PlanID: planID,
		ErsContext: internal.ERSContext{
			GlobalAccountID: globalAccountID,
			SubAccountID:    subAccountID,
		},
		PlatformRegion: options.ProvidePlatformRegion(),
		Parameters: internal.ProvisioningParametersDTO{
			Region: options.ProvideRegion(),
		},
	}

	instance := internal.Instance{
		RuntimeID:       runtimeID,
		ServicePlanID:   planID,
		ServicePlanName: planName,
		InstanceID:      instanceID,
		GlobalAccountID: globalAccountID,
		SubAccountID:    subAccountID,
		Parameters:      provisioningParameters,
		ProviderRegion:  *options.ProvideRegion(),
		InstanceDetails: internal.InstanceDetails{
			RuntimeID: runtimeID,
		},
	}

	provisioningOperation := internal.ProvisioningOperation{
		Operation: internal.Operation{
			State:                  domain.Succeeded,
			ID:                     uuid.New(),
			InstanceID:             instanceID,
			ProvisioningParameters: provisioningParameters,
			InstanceDetails: internal.InstanceDetails{
				RuntimeID: instance.RuntimeID,
			},
		},
	}
	shoot := &gardenerapi.Shoot{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      fmt.Sprintf("shoot%s", runtimeID),
			Namespace: s.gardenerNamespace,
			Labels: map[string]string{
				globalAccountLabel: globalAccountID,
				subAccountLabel:    subAccountID,
			},
			Annotations: map[string]string{
				runtimeIDAnnotation: runtimeID,
			},
		},
		Spec: gardenerapi.ShootSpec{
			Region: *options.ProvideRegion(),
			Maintenance: &gardenerapi.Maintenance{
				TimeWindow: &gardenerapi.MaintenanceTimeWindow{
					Begin: "030000+0000",
					End:   "040000+0000",
				},
			},
		},
	}

	require.NoError(s.t, s.storage.Instances().Insert(instance))
	require.NoError(s.t, s.storage.Operations().InsertProvisioningOperation(provisioningOperation))
	_, err := s.gardenerClient.CoreV1beta1().Shoots(s.gardenerNamespace).Create(shoot)
	require.NoError(s.t, err)
	return runtimeID
}

func (s *OrchestrationSuite) createOrchestration(oType orchestration.Type, queue *process.Queue, params orchestration.Parameters) string {
	now := time.Now()
	o := internal.Orchestration{
		OrchestrationID: uuid.New(),
		Type:            oType,
		State:           orchestration.Pending,
		Description:     "started processing of Kyma upgrade",
		Parameters:      params,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(s.t, s.storage.Orchestrations().Insert(o))

	queue.Add(o.OrchestrationID)
	return o.OrchestrationID
}

func (s *OrchestrationSuite) CreateUpgradeKymaOrchestration(params orchestration.Parameters) string {
	return s.createOrchestration(orchestration.UpgradeKymaOrchestration, s.kymaQueue, params)
}

func (s *OrchestrationSuite) CreateUpgradeClusterOrchestration(params orchestration.Parameters) string {
	return s.createOrchestration(orchestration.UpgradeClusterOrchestration, s.clusterQueue, params)
}

func (s *OrchestrationSuite) finishOperationByProvisioner(operationType gqlschema.OperationType, runtimeID string) {
	err := wait.Poll(time.Millisecond*100, 15*time.Second, func() (bool, error) {
		status := s.provisionerClient.FindOperationByRuntimeIDAndType(runtimeID, operationType)
		if status.ID != nil {
			s.provisionerClient.FinishProvisionerOperation(*status.ID)
			return true, nil
		}
		return false, nil
	})
	assert.NoError(s.t, err, "timeout waiting for provisioner operation to exist")
}

func (s *OrchestrationSuite) FinishUpgradeOperationByProvisioner(runtimeID string) {
	s.finishOperationByProvisioner(gqlschema.OperationTypeUpgrade, runtimeID)
}

func (s *OrchestrationSuite) FinishUpgradeShootOperationByProvisioner(runtimeID string) {
	s.finishOperationByProvisioner(gqlschema.OperationTypeUpgradeShoot, runtimeID)
}

func (s *OrchestrationSuite) WaitForOrchestrationState(orchestrationID string, state string) {
	var orchestration *internal.Orchestration
	err := wait.PollImmediate(100*time.Millisecond, 15*time.Second, func() (done bool, err error) {
		orchestration, _ = s.storage.Orchestrations().GetByID(orchestrationID)
		return orchestration.State == state, nil
	})
	assert.NoError(s.t, err, "timeout waiting for the orchestration expected state %s. The existing orchestration %+v", state, orchestration)
}

func (s *OrchestrationSuite) AssertRuntimeUpgraded(runtimeID string, version string) {
	assert.True(s.t, s.provisionerClient.IsRuntimeUpgraded(runtimeID, version), "The runtime %s expected to be upgraded", runtimeID)
}

func (s *OrchestrationSuite) AssertRuntimeNotUpgraded(runtimeID string) {
	assert.False(s.t, s.provisionerClient.IsRuntimeUpgraded(runtimeID, ""), "The runtime %s expected to be not upgraded", runtimeID)
}

func (s *OrchestrationSuite) AssertShootUpgraded(runtimeID string) {
	assert.True(s.t, s.provisionerClient.IsShootUpgraded(runtimeID), "The shoot %s expected to be upgraded", runtimeID)
}

func (s *OrchestrationSuite) AssertShootNotUpgraded(runtimeID string) {
	assert.False(s.t, s.provisionerClient.IsShootUpgraded(runtimeID), "The shoot %s expected to be not upgraded", runtimeID)
}

func fixK8sResources(defaultKymaVersion string, additionalKymaVersions []string) []runtime.Object {
	var resources []runtime.Object
	override := &coreV1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "overrides",
			Namespace: "kcp-system",
			Labels: map[string]string{
				fmt.Sprintf("overrides-version-%s", defaultKymaVersion): "true",
				"overrides-plan-azure": "true",
			},
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}
	for _, version := range additionalKymaVersions {
		override.ObjectMeta.Labels[fmt.Sprintf("overrides-version-%s", version)] = "true"
	}
	resources = append(resources, override)

	return resources
}
