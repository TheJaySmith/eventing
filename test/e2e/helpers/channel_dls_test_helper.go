/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

// ChannelDeadLetterSinkTestHelper is the helper function for channel_deadlettersink_test
func ChannelDeadLetterSinkTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	const (
		senderName    = "e2e-channelchain-sender"
		loggerPodName = "e2e-channel-dls-logger-pod"
	)
	channelNames := []string{"e2e-channel-dls"}
	// subscriptionNames corresponds to Subscriptions
	subscriptionNames := []string{"e2e-channel-dls-subs1"}

	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := common.Setup(st, true)
		defer common.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReady(&channel)

		// create loggerPod and expose it as a service
		pod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(pod, common.WithService(loggerPodName))

		// create subscriptions that subscribe to a service that does not exist
		client.CreateSubscriptionsOrFail(
			subscriptionNames,
			channelNames[0],
			&channel,
			resources.WithSubscriberForSubscription("does-not-exist"),
			resources.WithDeadLetterSinkForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the first channel
		body := fmt.Sprintf("TestChannelDeadLetterSink %s", uuid.NewUUID())
		event := &resources.CloudEvent{
			Source:   senderName,
			Type:     resources.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		if err := client.SendFakeEventToAddressable(senderName, channelNames[0], &channel, event); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelNames[0])
		}

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames)
		if err := client.CheckLog(loggerPodName, common.CheckerContainsCount(body, expectedContentCount)); err != nil {
			st.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", body, expectedContentCount, loggerPodName, err)
		}
	})
}
