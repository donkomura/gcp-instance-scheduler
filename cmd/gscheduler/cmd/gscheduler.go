/**
 * Copyright (c) 2019-present Future Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package cmd

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/future-architect/gcp-instance-scheduler/operator"

	"cloud.google.com/go/pubsub"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/net/context"
)

// Operation target label name
const TargetLabel = "state-scheduler"

// API call interval
const ShutdownInterval = 50 * time.Millisecond

type SubscribedMessage struct {
	Command string `json:"command"`
}

func ReceiveEvent(ctx context.Context, msg *pubsub.Message, flags Flags) error {
	projectID := flags.Project

	if len(projectID) == 0 {
		projectID = os.Getenv("GCP_PROJECT")
	}
	log.Printf("Project ID: %v", projectID)

	// decode the json message from Pub/Sub
	message, err := decode(msg.Data)
	if err != nil {
		log.Printf("Error at the fucntion 'DecodeMessage': %v", err)
	}
	log.Printf("Subscribed message(Command): %v", message.Command)

	// for multierror
	var errorLog error

	if err := operator.SetLabelNodePoolSize(ctx, projectID, TargetLabel, ShutdownInterval); err != nil {
		errorLog = multierror.Append(errorLog, err)
		log.Printf("Error in setting labels on GKE cluster: %v", err)
	}

	// show cluster status
	if err := operator.ShowClusterStatus(ctx, projectID, TargetLabel); err != nil {
		errorLog = multierror.Append(errorLog, err)
		log.Printf("Error in stopping GKE: %v", err)
	}

	rpt, err := operator.InstanceGroupResource(ctx, projectID).
		FilterLabel(TargetLabel, true).
		ShutdownWithInterval(ctx, ShutdownInterval)
	if err != nil {
		errorLog = multierror.Append(errorLog, err)
		log.Printf("Some error occured in stopping gce instances: %v", err)
	}
	rpt.Show()

	rpt, err = operator.ComputeEngineResource(ctx, projectID).
		FilterLabel(TargetLabel, true).
		ShutdownWithInterval(ctx, ShutdownInterval)
	if err != nil {
		errorLog = multierror.Append(errorLog, err)
		log.Printf("Some error occured in stopping gce instances: %v", err)
	}
	rpt.Show()

	rpt, err = operator.SQLResource(ctx, projectID).
		FilterLabel(TargetLabel, true).
		ShutdownWithInterval(ctx, ShutdownInterval)
	if err != nil {
		errorLog = multierror.Append(errorLog, err)
		log.Printf("Some error occured in stopping sql instances: %v", err)
	}
	rpt.Show()

	log.Printf("done.")
	return errorLog
}

func decode(payload []byte) (msgData SubscribedMessage, err error) {
	if err = json.Unmarshal(payload, &msgData); err != nil {
		log.Printf("Message[%v] ... Could not decode subscribing data: %v", payload, err)
		if e, ok := err.(*json.SyntaxError); ok {
			log.Printf("syntax error at byte offset %d", e.Offset)
		}
		return
	}
	return
}
