package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/objx"
)

func populateInventory(i *integration.Integration) {
	// TODO lookup local node to append inventory to
	entity, err := i.Entity("local", "local")
	if err != nil {
		logger.Errorf("couldn't create local entity: %v", err)
		return
	}

	err = populateConfigInventory(entity)
	if err != nil {
		logger.Errorf("couldn't populate config inventory: %v", err)
	}

	localNode, err := getLocalNode()
	if err != nil {
		logger.Errorf("couldn't get local node stats: %v", err)
		return
	}

	populateNodeStatInventory(entity, localNode)
}

func readConfigFile(filePath string) (map[string]interface{}, error) {
	rawYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		logger.Errorf("could not open specified config file: %v", err)
		return nil, err
	}

	parsedYaml := make(map[string]interface{})

	err = yaml.Unmarshal(rawYaml, parsedYaml)
	if err != nil {
		logger.Errorf("could not parse configuration yaml: %v", err)
		return nil, err
	}

	return parsedYaml, nil
}

func populateConfigInventory(entity *integration.Entity) error {
	configYaml, err := readConfigFile(args.ConfigPath)
	if err != nil {
		return err
	}

	for key, value := range configYaml {
		err = entity.SetInventoryItem("config", key, value)
		if err != nil {
			logger.Errorf("could not set inventory item: %v", err)
		}
	}
	return nil
}

func populateNodeStatInventory(entity *integration.Entity, localNode objx.Map) {
	parseProcessStats(entity, localNode)
	parsePluginsAndModules(entity, localNode)
	parseNodeIngests(entity, localNode)
}

func getLocalNode() (objx.Map, error) {
	client, err := NewClient(nil)
	if err != nil {
		return nil, err
	}

	localNodeStats, err := client.Request(localNodeInventoryEndpoint)
	if err != nil {
		return nil, err
	}

	return parseLocalNode(localNodeStats)
}

func parseLocalNode(nodeStats objx.Map) (objx.Map, error) {
	nodes := nodeStats.Get("nodes").ObjxMap()
	if len(nodes) == 1 {
		for _, v := range nodes {
			return objx.New(v), nil
		}
	}
	return nil, fmt.Errorf("could not identify local node")
}

func parseNodeIngests(entity *integration.Entity, stats objx.Map) []string {
	processorList := stats.Get("ingest.processors").ObjxMapSlice()

	typeList := []string{}

	for _, processor := range processorList {
		ingestType := processor.Get("type").String()
		typeList = append(typeList, ingestType)
	}

	err := entity.SetInventoryItem("config", "ingest", strings.Join(typeList, ","))
	if err != nil {
		logger.Errorf("error setting ingest types: %v", err)
	}

	return typeList
}

func parseProcessStats(entity *integration.Entity, stats objx.Map) {
	processStats := stats.Get("process").ObjxMap()

	for k, v := range processStats {
		err := entity.SetInventoryItem("config", "process."+k, v)
		if err != nil {
			logger.Errorf("error setting inventory item [%v -> %v]: %v", k, v, err)
		}
	}
}

func parsePluginsAndModules(entity *integration.Entity, stats objx.Map) {

	fieldNames := []string{
		"version",
		"elasticsearch_version",
		"java_version",
		"description",
		"classname",
	}

	for _, addonType := range []string{"plugins", "modules"} {
		addonStats := stats.Get(addonType).ObjxMapSlice()
		for _, addon := range addonStats {
			addonName := addon.Get("name").Str()
			for _, field := range fieldNames {
				inventoryKey := fmt.Sprintf("%v.%v.%v", addonType, addonName, field)
				inventoryValue := addon.Get(field).Str()
				err := entity.SetInventoryItem("config", inventoryKey, inventoryValue)
				if err != nil {
					logger.Errorf("error setting inventory item [%v -> %v]: %v", inventoryKey, inventoryValue, err)
				}
			}
		}
	}
}
