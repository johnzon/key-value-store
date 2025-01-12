package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

//Comunicator interface
//We are using http for now but having an interface will enable us to swap http with gprc
type NodeCommunicator interface {
	SendHeartbeat(url string, nodeID string, term int, followers map[string]string) error
	PostHTTPRequest(url string, payload map[string]interface{}) ([]byte, error)
}

// HTTPCommunicator implements the Communicator interface using HTTP.
type HTTPCommunicator struct {
	Client *http.Client
}

// HeartbeatRequest represents the payload sent in a heartbeat message.
type HeartbeatRequest struct {
	NodeID    string            `json:"node_id"`
	Term      int               `json:"term"`
	URL       string            `json:"url"`
	Followers map[string]string `json:"followers"`
}

// NewHTTPCommunicator creates a new HTTP communicator.
func NewHTTPCommunicator() *HTTPCommunicator {
	return &HTTPCommunicator{
		Client: &http.Client{},
	}
}

// SendHeartbeat sends a heartbeat message to the given node with metadata.
func (h *HTTPCommunicator) SendHeartbeat(url string, nodeID string, term int, follower map[string]string) error {
	// Create the payload
	payload := HeartbeatRequest{
		NodeID: nodeID,
		Term:   term,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling heartbeat payload: %v", err)
	}

	// Send POST request
	resp, err := h.Client.Post(
		fmt.Sprintf("http://%s/heartbeat", url),
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("error sending heartbeat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat request failed with status: %s", resp.Status)
	}

	return nil
}

// PostHTTPRequest is a helper function to send HTTP POST requests and return the response body.
func (h *HTTPCommunicator) PostHTTPRequest(url string, payload map[string]interface{}) ([]byte, error) {
	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Send the HTTP POST request
	resp, err := h.Client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check the HTTP response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return body, nil
}
