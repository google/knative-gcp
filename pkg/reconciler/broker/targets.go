package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/knative-gcp/pkg/broker/config"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/eventing/pkg/logging"
)

const (
	targetsCMKey          = "targets"
	targetsFileName       = "targets.txt"
	targetsCMResyncPeriod = 10 * time.Second
)

type BrokerConfigUpdater interface {
	UpdateBrokerConfig(context.Context, *config.Broker) error
}

type updateReq struct {
	broker  *config.Broker
	replyCh chan error
}

type targetsReconciler struct {
	targetsName, targetsNS string
	targets                *config.TargetsConfig
	updateCh               chan updateReq
	configMaps             v1.ConfigMapInterface
	logger                 *zap.Logger
}

func NewTargetsReconciler(ctx context.Context, client kubernetes.Interface, targetsNS, targetsName string) BrokerConfigUpdater {
	r := &targetsReconciler{
		targetsName: targetsName,
		targetsNS:   targetsNS,
		updateCh:    make(chan updateReq),
		configMaps:  client.CoreV1().ConfigMaps(targetsNS),
		logger:      logging.FromContext(ctx),
	}
	if err := r.loadTargetsConfig(ctx); err != nil {
		r.logger.Error("error loading targets config", zap.Error(err))
		r.targets = new(config.TargetsConfig)
	}
	go r.reconcile(ctx)
	return r
}

func (r *targetsReconciler) UpdateBrokerConfig(ctx context.Context, b *config.Broker) error {
	if b.Name == "" || b.Namespace == "" {
		return fmt.Errorf("broker missing namespace or name: %s", prototext.MarshalOptions{}.Format(b))
	}

	req := updateReq{
		broker:  b,
		replyCh: make(chan error, 1),
	}
	select {
	case r.updateCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.replyCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// loadTargetsConfig retrieves the targets ConfigMap and populates r.targets.
func (r *targetsReconciler) loadTargetsConfig(ctx context.Context) error {
	logger := r.logger.With(zap.String("targetsName", r.targetsName), zap.String("targetsNS", r.targetsNS))
	logger.Debug("Loading targets config from configmap")
	targetsConfigMap, err := r.configMaps.Get(r.targetsName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			r.targets = &config.TargetsConfig{
				Brokers: make(map[string]*config.Broker),
			}
			return nil
		}
		return fmt.Errorf("error getting targets ConfigMap: %w", err)
	}

	targets := new(config.TargetsConfig)
	if err := proto.Unmarshal(targetsConfigMap.BinaryData[targetsCMKey], targets); err != nil {
		return err
	}
	r.logger.Debug("Loaded targets config from ConfigMap",
		zap.String("resourceVersion", targetsConfigMap.ResourceVersion))

	r.targets = targets
	return nil
}

func (r *targetsReconciler) reconcile(ctx context.Context) {
	var writing, waiting []chan<- error
	var writeCh <-chan error
	var lastErr error
	for {
		var resync <-chan time.Time
		var resyncTimer *time.Timer
		if writeCh == nil {
			resyncTimer = time.NewTimer(targetsCMResyncPeriod)
			resync = resyncTimer.C
		}

		select {
		case req := <-r.updateCh:
			k := config.BrokerKey(req.broker.Namespace, req.broker.Name)
			if proto.Equal(req.broker, r.targets.Brokers[k]) {
				if waiting != nil {
					waiting = append(waiting, req.replyCh)
					break
				}
				if writeCh != nil {
					writing = append(writing, req.replyCh)
					break
				}
				if lastErr == nil {
					req.replyCh <- nil
					break
				}
			}
			r.targets.GetBrokers()[k] = req.broker
			if writeCh != nil {
				waiting = append(waiting, req.replyCh)
				break
			}
			writeCh = r.startWrite()
			writing = append(writing, req.replyCh)

		case lastErr = <-writeCh:
			writeCh = nil
			for _, ch := range writing {
				ch <- lastErr
			}
			writing, waiting = waiting, nil
			if writing == nil {
				break
			}
			writeCh = r.startWrite()

		case <-resync:
			writeCh = r.startWrite()

		case <-ctx.Done():
			r.logger.Info("targets reconciler stopped", zap.Error(ctx.Err()))
			if resyncTimer != nil {
				resyncTimer.Stop()
			}
			return
		}

		if resyncTimer != nil {
			resyncTimer.Stop()
		}
	}
}

func (r *targetsReconciler) startWrite() <-chan error {
	writeCh := make(chan error, 1)
	go r.writeTargets(writeCh, proto.Clone(r.targets).(*config.TargetsConfig))
	return writeCh
}

func (r *targetsReconciler) writeTargets(replyCh chan<- error, targets *config.TargetsConfig) (err error) {
	defer func() {
		replyCh <- err
	}()

	r.logger.Debug("writing targets config", zap.Any("targetsConfig", targets))

	targetsData, err := proto.Marshal(targets)
	if err != nil {
		return
	}
	targetsText := prototext.Format(targets)

	targetsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.targetsName,
			Namespace: r.targetsNS,
		},
		BinaryData: map[string][]byte{targetsCMKey: targetsData},
		// Write out the text version for debugging purposes only
		Data: map[string]string{targetsFileName: targetsText},
	}
	_, err = r.configMaps.Update(targetsConfigMap)
	if errors.IsNotFound(err) {
		_, err = r.configMaps.Create(targetsConfigMap)
	}
	return
}
