package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var NodeTestFile = filepath.Join("testdata", "good-nodes-local.json")

type mockClient struct{
	mock.Mock
}

func (mc mockClient) Request(endpoint string, responseObject interface{}) error {
	param := mc.Called(endpoint).String(0)
	if param == "error" {
		return fmt.Errorf("client error")
	}

	fileData, _ := ioutil.ReadFile(param)
	_ = json.Unmarshal(fileData, responseObject)
	return nil
}

func TestReadConfigFile(t *testing.T) {
	testCases := []struct {
		filePath    string
		expectedMap map[string]interface{}
	}{
		{
			filepath.Join("testdata", "elasticsearch_sample.yml"),
			map[string]interface{}{
				"path.data":    "/var/lib/elasticsearch",
				"path.logs":    "/var/log/elasticsearch",
				"network.host": "0.0.0.0",
			},
		},
	}

	for _, tc := range testCases {
		setupTestArgs()
		resultMap, err := readConfigFile(tc.filePath)
		if err != nil {
			t.Errorf("couldn't read config file: %v", err)
		} else {
			if expected := reflect.DeepEqual(tc.expectedMap, resultMap); !expected {
				t.Errorf("maps didn't match")
			}
		}
	}
}

func TestConfigErrors(t *testing.T) {
	testCases := []struct {
		filePath string
	}{
		{
			filepath.Join("testdata", "elasticsearch_doesntexist.yml"),
		},
		{
			filepath.Join("testdata", "elasticsearch_bad.yml"),
		},
	}

	for _, tc := range testCases {
		setupTestArgs()
		_, err := readConfigFile(tc.filePath)
		if err == nil {
			t.Errorf("was not expecting a result")
		}
	}
}

func TestPopulateConfigInventory(t *testing.T) {
	i, e := getTestingEntity(t)

	dataPath := filepath.Join("testdata", "elasticsearch_sample.yml")
	goldenPath := dataPath + ".golden"

	args.ConfigPath = dataPath

	populateConfigInventory(e)

	actual, _ := i.MarshalJSON()

	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actual, 0644)
		assert.NoError(t, err)
	}

	expected, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expected, actual)
}

func TestPopulateConfigInventoryWithBadFilename(t *testing.T) {
	_, e := getTestingEntity(t)

	dataPath := filepath.Join("testdata", "elasticsearch_doesntexist.yml")
	args.ConfigPath = dataPath

	err := populateConfigInventory(e)
	assert.Error(t, err)
}

func TestParsePluginsAndModules(t *testing.T) {
	i, e := getTestingEntity(t)

	dataPath := filepath.Join("testdata", "good-node.json")
	goldenPath := dataPath + ".golden"

	statsFromFile, _ := ioutil.ReadFile(dataPath)
	responseObject := new(LocalNode)
	_ = json.Unmarshal(statsFromFile, &responseObject)

	populateNodeStatInventory(e, responseObject)

	actualJSON, err := i.MarshalJSON()
	assert.NoError(t, err)

	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actualJSON, 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expectedJSON, actualJSON)
}

func TestGetLocalNode(t *testing.T) {
	goldenPath := filepath.Join("testdata", "good-nodes-local.json.golden")

	fakeClient := mockClient{}
	mockedReturnVal := filepath.Join("testdata", "good-nodes-local.json")
	fakeClient.On("Request", "/_nodes/_local").Return(mockedReturnVal, nil).Once()

	resultName, resultStats, _ := getLocalNode(fakeClient)
	assert.Equal(t, "z9ZPp87vT92qG1cRVRIcMQ", resultName)

	actualString, _ := json.Marshal(resultStats)
	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, []byte(actualString), 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, string(expectedJSON), string(actualString))
	fakeClient.AssertExpectations(t)
}

func TestGetLocalNodeWithBadNodeResponse(t *testing.T) {
	fakeClient := mockClient{}
	mockedReturnVal := "error"
	fakeClient.On("Request", "/_nodes/_local").Return(mockedReturnVal, nil).Once()

	resultName, resultObject, err := getLocalNode(fakeClient)
	assert.Equal(t, "", resultName)
	assert.Nil(t, resultObject)
	assert.Error(t, err)
}

func TestGetLocalNodeWithMultipleNodes(t *testing.T) {
	fakeClient := mockClient{}
	mockedReturnVal := filepath.Join("testdata", "bad-nodes-local.json")
	fakeClient.On("Request", "/_nodes/_local").Return(mockedReturnVal, nil).Once()

	resultName, resultStats, err := getLocalNode(fakeClient)
	assert.Equal(t, "", resultName)
	assert.Nil(t, resultStats)
	assert.Error(t, err)
}

func TestPopulateInventory(t *testing.T) {
	setupTestArgs()
	args.ConfigPath = filepath.Join("testdata", "elasticsearch_sample.yml")

	goldenPath := filepath.Join("testdata", "good-inventory.json.golden")

	fakeClient := mockClient{}
	mockedReturnVal := filepath.Join("testdata", "good-nodes-local.json")
	fakeClient.On("Request", "/_nodes/_local").Return(mockedReturnVal, nil).Once()

	i := getTestingIntegration(t)
	populateInventory(i, fakeClient)

	actualJSON, _ := i.MarshalJSON()
	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actualJSON, 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expectedJSON, actualJSON)
	fakeClient.AssertExpectations(t)
}

func TestParseProcessStatsWithIncorrectTypes(t *testing.T) {
	dataPath := filepath.Join("testdata", "bad-process-stats.json")
	goldenPath := dataPath + ".golden"

	jsonBytes, _ := ioutil.ReadFile(dataPath)
	nodeObject := new(LocalNode)
	_ = json.Unmarshal(jsonBytes, nodeObject)

	i, e := getTestingEntity(t)

	parseProcessStats(e, nodeObject)

	actualJSON, _ := i.MarshalJSON()
	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actualJSON, 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expectedJSON, actualJSON)
}

func TestParseProcessStatsWithEmptyStats(t *testing.T) {
	dataPath := filepath.Join("testdata", "empty-process-stats.json")
	goldenPath := dataPath + ".golden"

	jsonBytes, _ := ioutil.ReadFile(dataPath)
	nodeObject := new(LocalNode)
	_ = json.Unmarshal(jsonBytes, nodeObject)

	i, e := getTestingEntity(t)

	parseProcessStats(e, nodeObject)

	actualJSON, _ := i.MarshalJSON()
	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actualJSON, 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expectedJSON, actualJSON)
}

func TestParseProcessStatsWithMissingProcessStats(t *testing.T) {
	dataPath := filepath.Join("testdata", "missing-process-stats.json")
	goldenPath := dataPath + ".golden"

	jsonBytes, _ := ioutil.ReadFile(dataPath)
	nodeObject := new(LocalNode)
	_ = json.Unmarshal(jsonBytes, nodeObject)

	i, e := getTestingEntity(t)

	parseProcessStats(e, nodeObject)

	actualJSON, _ := i.MarshalJSON()
	if *update {
		t.Log("Writing .golden file")
		err := ioutil.WriteFile(goldenPath, actualJSON, 0644)
		assert.NoError(t, err)
	}

	expectedJSON, _ := ioutil.ReadFile(goldenPath)

	assert.Equal(t, expectedJSON, actualJSON)
}