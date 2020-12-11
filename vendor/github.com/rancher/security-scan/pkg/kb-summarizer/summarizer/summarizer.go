package summarizer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	kb "github.com/aquasecurity/kube-bench/check"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	DefaultOutputFileName       = "report.json"
	DefaultControlsDirectory    = "/etc/kube-bench/cfg"
	VersionMappingKey           = "version_mapping"
	ConfigFilename              = "config.yaml"
	MasterControlsFilename      = "master.yaml"
	EtcdControlsFilename        = "etcd.yaml"
	NodeControlsFilename        = "node.yaml"
	MasterResultsFilename       = "master.json"
	EtcdResultsFilename         = "etcd.json"
	NodeResultsFilename         = "node.json"
	ControlPlaneResultsFilename = "controlplane.json"
	PoliciesResultsFilename     = "policies.json"
	CurrentBenchmarkKey         = "current"
	DefaultErrorLogFileName     = "error.log"
)

type Summarizer struct {
	// mapping for k8s version to default benchmark version
	kubeToBenchmarkMap map[string]string
	BenchmarkVersion   string
	ControlsDirectory  string
	InputDirectory     string
	OutputDirectory    string
	OutputFilename     string
	FailuresOnly       bool
	fullReport         *SummarizedReport
	groupWrappersMap   map[string]*GroupWrapper
	checkWrappersMaps  map[string]*CheckWrapper
	userSkip           map[string]bool
	defaultSkip        map[string]string
	notApplicable      map[string]string
	nodeSeen           map[NodeType]map[string]bool
}

type State string

const (
	Pass          State = "P"
	Fail          State = "F"
	Skip          State = "S"
	Mixed         State = "M"
	NotApplicable State = "N"
	Warn          State = "W"

	SKIP kb.State = "SKIP"
	NA   kb.State = "NA"

	CheckTypeSkip = "skip"
)

type NodeType string

const (
	NodeTypeNone   NodeType = ""
	NodeTypeEtcd   NodeType = "e"
	NodeTypeMaster NodeType = "m"
	NodeTypeNode   NodeType = "n"
)

type CheckWrapper struct {
	ID                 string                       `yaml:"id" json:"id"`
	Text               string                       `json:"d"`
	Type               string                       `json:"tt"`
	Remediation        string                       `json:"r"`
	State              State                        `json:"s"`
	Scored             bool                         `json:"sc"`
	Result             map[kb.State]map[string]bool `json:"-"`
	NodeType           []NodeType                   `json:"t"`
	NodesMap           map[string]bool              `json:"-"`
	Nodes              []string                     `json:"n,omitempty"`
	Audit              string                       `json:"a"`
	AuditConfig        string                       `json:"ac"`
	TestInfo           []string                     `json:"ti"`
	Commands           []*exec.Cmd                  `json:"c"`
	ConfigCommands     []*exec.Cmd                  `json:"cc"`
	ActualValueNodeMap map[string]string            `json:"avmap"`
	ExpectedResult     string                       `json:"er"`
}

type GroupWrapper struct {
	ID            string          `yaml:"id" json:"id"`
	Text          string          `json:"d"`
	CheckWrappers []*CheckWrapper `json:"o"`
}

type SummarizedReport struct {
	Version       string                `json:"v"`
	Total         int                   `json:"t"`
	Fail          int                   `json:"f"`
	Pass          int                   `json:"p"`
	Warn          int                   `json:"w"`
	Skip          int                   `json:"s"`
	NotApplicable int                   `json:"na"`
	Nodes         map[NodeType][]string `json:"n"`
	GroupWrappers []*GroupWrapper       `json:"o"`
}

type skipConfig struct {
	Skip map[string][]string `json:"skip"`
}

var controlFilesToIgnore = map[string]bool{
	"config.yaml": true,
}

func NewSummarizer(
	k8sVersion,
	benchmarkVersion,
	controlsDir,
	inputDir,
	outputDir,
	outputFilename,
	userSkipConfigFile,
	defaultSkipConfigFile,
	notApplicableConfigFile string,
	failuresOnly bool,
) (*Summarizer, error) {
	var err error
	s := &Summarizer{
		ControlsDirectory: controlsDir,
		InputDirectory:    inputDir,
		OutputDirectory:   outputDir,
		OutputFilename:    outputFilename,
		FailuresOnly:      failuresOnly,
		fullReport: &SummarizedReport{
			Nodes:         map[NodeType][]string{},
			GroupWrappers: []*GroupWrapper{},
		},
		groupWrappersMap:  map[string]*GroupWrapper{},
		checkWrappersMaps: map[string]*CheckWrapper{},
		nodeSeen:          map[NodeType]map[string]bool{},
	}
	if err := s.loadVersionMapping(); err != nil {
		return nil, fmt.Errorf("error loading version mapping: %v", err)
	}
	if benchmarkVersion != "" {
		s.BenchmarkVersion = benchmarkVersion
	} else {
		s.BenchmarkVersion, err = s.getBenchmarkFor(k8sVersion)
		if err != nil {
			return nil, fmt.Errorf("error getting benchmarkVersion for k8s version %v: %v", k8sVersion, err)
		}
	}

	userSkip, err := GetUserSkipInfo(s.BenchmarkVersion, userSkipConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error getting user skip info: %v", err)
	}
	s.userSkip = userSkip

	defaultSkip, err := GetChecksMapFromConfigFile(defaultSkipConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error getting default skip info: %v", err)
	}
	s.defaultSkip = defaultSkip

	notApplicable, err := GetChecksMapFromConfigFile(notApplicableConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error getting default skip info: %v", err)
	}
	s.notApplicable = notApplicable

	if err := s.loadControls(); err != nil {
		return nil, fmt.Errorf("error loading controls: %v", err)
	}
	return s, nil
}

func GetUserSkipInfo(benchmark, skipConfigFile string) (map[string]bool, error) {
	skipMap := map[string]bool{}
	sc := &skipConfig{}
	if skipConfigFile == "" {
		return skipMap, nil
	}
	data, err := ioutil.ReadFile(skipConfigFile)
	if err != nil {
		return skipMap, fmt.Errorf("error reading file %v: %v", skipConfigFile, err)
	}
	err = json.Unmarshal(data, sc)
	if err != nil {
		return skipMap, fmt.Errorf("error unmarshalling skip str: %v", err)
	}
	skipArr, ok := sc.Skip[benchmark]
	if !ok {
		skipArr = sc.Skip[CurrentBenchmarkKey]
	}
	if len(skipArr) == 0 {
		return skipMap, nil
	}
	for _, v := range skipArr {
		skipMap[v] = true
	}
	logrus.Debugf("skipMap: %+v", skipMap)
	return skipMap, nil
}

func GetChecksMapFromConfigFile(configFile string) (map[string]string, error) {
	checksMap := map[string]string{}
	if configFile == "" {
		return checksMap, nil
	}
	logrus.Infof("loading checks from config file: %v", configFile)
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return checksMap, fmt.Errorf("error reading file %v: %v", configFile, err)
	}
	if len(data) == 0 {
		return checksMap, nil
	}
	if err := json.Unmarshal(data, &checksMap); err != nil {
		return nil, fmt.Errorf("error unmarshalling config file %v: %v", configFile, err)
	}
	return checksMap, nil
}

func (s *Summarizer) getBenchmarkFor(k8sVersion string) (string, error) {
	if k8sVersion == "" {
		return "", nil
	}
	b, ok := s.kubeToBenchmarkMap[k8sVersion]
	if !ok {
		return "", fmt.Errorf("k8s version: %v not supported", k8sVersion)
	}
	return b, nil
}

func (s *Summarizer) processOneResultFileForHost(results *kb.Controls, hostname string) {
	for _, group := range results.Groups {
		for _, check := range group.Checks {
			logrus.Infof("host:%s id: %s %v", hostname, check.ID, check.State)
			printCheck(check)
			cw := s.checkWrappersMaps[check.ID]
			if cw == nil {
				logrus.Errorf("check %s found in results but not in spec", check.ID)
				continue
			}
			if check.Type == CheckTypeSkip {
				check.State = NA
			}
			if msg, ok := s.notApplicable[check.ID]; ok {
				check.State = NA
				check.Remediation = msg
			} else if msg, ok := s.defaultSkip[check.ID]; ok {
				check.State = SKIP
				check.Remediation = msg
			} else if s.userSkip[check.ID] {
				check.State = SKIP
			}
			if cw.Result[check.State] == nil {
				cw.Result[check.State] = make(map[string]bool)
			}
			cw.Result[check.State][hostname] = true

			if cw.ActualValueNodeMap == nil {
				cw.ActualValueNodeMap = make(map[string]string)
			}
			cw.ActualValueNodeMap[hostname] = check.ActualValue

			resultCheckWrapper := getCheckWrapper(check)
			resultCheckWrapper.Result = cw.Result
			resultCheckWrapper.ActualValueNodeMap = cw.ActualValueNodeMap
			s.checkWrappersMaps[check.ID] = resultCheckWrapper
		}
	}
}

func (s *Summarizer) addNode(nodeType NodeType, hostname string) {
	if nodeType == NodeTypeNone {
		return
	}
	if !s.nodeSeen[nodeType][hostname] {
		s.nodeSeen[nodeType][hostname] = true
		s.fullReport.Nodes[nodeType] = append(s.fullReport.Nodes[nodeType], hostname)
	}
}

func (s *Summarizer) summarizeForHost(hostname string) error {
	logrus.Debugf("summarizeForHost: %s", hostname)

	hostDir := fmt.Sprintf("%s/%s", s.InputDirectory, hostname)

	resultFilesPaths, err := filepath.Glob(fmt.Sprintf("%s/*.json", hostDir))
	if err != nil {
		return fmt.Errorf("error globing files: %v", err)
	}

	nodeTypeMapping := getResultsFileNodeTypeMapping()

	for _, resultFilePath := range resultFilesPaths {
		resultFile := filepath.Base(resultFilePath)
		nodeType, ok := nodeTypeMapping[resultFile]
		if !ok {
			logrus.Errorf("unknown result file found: %s", resultFilePath)
			continue
		}
		s.addNode(nodeType, hostname)
		logrus.Debugf("host: %s resultFile: %s", hostname, resultFile)
		// Load one result file
		// Marshal it into the results
		contents, err := ioutil.ReadFile(filepath.Clean(resultFilePath))
		if err != nil {
			return fmt.Errorf("error reading file %+s: %v", resultFilePath, err)
		}

		results := &kb.Controls{}
		if err := json.Unmarshal(contents, results); err != nil {
			return fmt.Errorf("error unmarshalling: %v", err)
		}
		logrus.Debugf("results: %+v", results)

		s.processOneResultFileForHost(results, hostname)
	}
	return nil
}

func (s *Summarizer) save() error {
	if _, err := os.Stat(s.OutputDirectory); os.IsNotExist(err) {
		if err2 := os.Mkdir(s.OutputDirectory, 0755); err2 != nil {
			return fmt.Errorf("error creating output directory: %v", err)
		}
	}
	outputFilePath := fmt.Sprintf("%s/%s", s.OutputDirectory, s.OutputFilename)
	jsonFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("error creating file %v: %v", outputFilePath, err)
	}
	jsonWriter := io.Writer(jsonFile)
	encoder := json.NewEncoder(jsonWriter)
	encoder.SetIndent("", " ")
	err = encoder.Encode(s.fullReport)
	if err != nil {
		return fmt.Errorf("error encoding: %v", err)
	}
	logrus.Infof("successfully saved report file: %v", outputFilePath)
	return nil
}

func (s *Summarizer) loadVersionMapping() error {
	configFileName := fmt.Sprintf("%s/%s", s.ControlsDirectory, ConfigFilename)
	v := viper.New()
	v.SetConfigFile(configFileName)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading in config file: %v", err)
	}

	kubeToBenchmarkMap := v.GetStringMapString(VersionMappingKey)
	if kubeToBenchmarkMap == nil || (len(kubeToBenchmarkMap) == 0) {
		return fmt.Errorf("config file is missing '%v' section", VersionMappingKey)
	}
	logrus.Debugf("%v: %v", VersionMappingKey, kubeToBenchmarkMap)
	s.kubeToBenchmarkMap = kubeToBenchmarkMap
	logrus.Infof("CONFIG: %+v\n", kubeToBenchmarkMap)
	return nil
}

func (s *Summarizer) loadControlsFromFile(filePath string) (*kb.Controls, error) {
	controls := &kb.Controls{}
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file %+v: %v", filePath, err)
	}
	if err := yaml.Unmarshal(fileContents, controls); err != nil {
		return nil, fmt.Errorf("error unmarshalling master controls file: %v", err)
	}
	logrus.Debugf("filePath: %v, controls: %+v", filePath, controls)
	return controls, nil
}

// func getNodeTypes() []NodeType {
// 	return []NodeType{
// 		NodeTypeMaster,
// 		NodeTypeEtcd,
// 		NodeTypeNode,
// 	}
// }

func getResultsFileNodeTypeMapping() map[string]NodeType {
	return map[string]NodeType{
		MasterResultsFilename:       NodeTypeMaster,
		EtcdResultsFilename:         NodeTypeEtcd,
		NodeResultsFilename:         NodeTypeNode,
		ControlPlaneResultsFilename: NodeTypeNone,
		PoliciesResultsFilename:     NodeTypeNone,
	}
}

func (s *Summarizer) getControlsFilePath(filename string) string {
	return fmt.Sprintf("%s/%s/%s", s.ControlsDirectory, s.BenchmarkVersion, filename)
}

func (s *Summarizer) getNodeTypeControlsFileMapping() map[string]NodeType {
	// load node type files first
	filepaths := map[string]NodeType{
		s.getControlsFilePath(MasterControlsFilename): NodeTypeMaster,
		s.getControlsFilePath(EtcdControlsFilename):   NodeTypeEtcd,
		s.getControlsFilePath(NodeControlsFilename):   NodeTypeNode,
	}
	allFiles, err := filepath.Glob(fmt.Sprintf("%s/%s/*.yaml", s.ControlsDirectory, s.BenchmarkVersion))
	if err != nil {
		logrus.Errorf("error globing files: %v", err)
		return filepaths
	}
	for _, f := range allFiles {
		if controlFilesToIgnore[f] {
			continue
		}
		if _, ok := filepaths[f]; ok {
			continue
		}
		filepaths[f] = NodeTypeNone
	}
	return filepaths
}

func (s *Summarizer) loadControls() error {
	var ok bool
	controlsFiles := s.getNodeTypeControlsFileMapping()

	var groupWrappers []*GroupWrapper
	for controlsFile, nodeType := range controlsFiles {
		s.nodeSeen[nodeType] = map[string]bool{}
		controls, err := s.loadControlsFromFile(controlsFile)
		if err != nil {
			logrus.Errorf("error loading controls from file %s: %v", controlsFile, err)
			continue
		}
		for _, g := range controls.Groups {
			var gw *GroupWrapper
			if gw, ok = s.groupWrappersMap[g.ID]; !ok {
				gw = getGroupWrapper(g)
				groupWrappers = append(groupWrappers, gw)
				s.groupWrappersMap[g.ID] = gw
			}
			for _, check := range g.Checks {
				if check.Type == CheckTypeSkip {
					check.State = NA
				}
				if msg, ok := s.notApplicable[check.ID]; ok {
					check.State = NA
					check.Remediation = msg
				} else if msg, ok := s.defaultSkip[check.ID]; ok {
					check.State = SKIP
					check.Remediation = msg
				}
				if cw, ok := s.checkWrappersMaps[check.ID]; !ok {
					s.fullReport.Total++
					c := getCheckWrapper(check)
					c.NodeType = []NodeType{nodeType}
					gw.CheckWrappers = append(gw.CheckWrappers, c)
					s.checkWrappersMaps[check.ID] = c
				} else {
					cw.NodeType = append(cw.NodeType, nodeType)
				}
			}
		}
	}

	sort.Slice(groupWrappers, func(i, j int) bool {
		return groupWrappers[i].ID < groupWrappers[j].ID
	})
	s.fullReport.GroupWrappers = groupWrappers
	logrus.Debugf("total groups loaded: %v", len(s.fullReport.GroupWrappers))
	logrus.Debugf("total controls loaded: %v", s.fullReport.Total)
	return nil
}

func getGroupWrapper(group *kb.Group) *GroupWrapper {
	return &GroupWrapper{
		ID:            group.ID,
		Text:          group.Text,
		CheckWrappers: []*CheckWrapper{},
	}
}

func getMappedState(state kb.State) State {
	switch state {
	case kb.PASS:
		return Pass
	case kb.FAIL:
		return Fail
	case kb.WARN:
		return Warn
	case kb.INFO:
		return NotApplicable
	case SKIP:
		return Skip
	case NA:
		return NotApplicable
	}
	return Fail
}

func getCheckWrapper(check *kb.Check) *CheckWrapper {
	return &CheckWrapper{
		ID:             check.ID,
		Text:           check.Text,
		Type:           check.Type,
		Remediation:    check.Remediation,
		Scored:         check.Scored,
		Result:         map[kb.State]map[string]bool{},
		Audit:          check.Audit,
		AuditConfig:    check.AuditConfig,
		TestInfo:       check.TestInfo,
		Commands:       check.Commands,
		ConfigCommands: check.ConfigCommands,
		ExpectedResult: check.ExpectedResult,
	}
}

func (s *Summarizer) getNodesMapOfCheckWrapper(check *CheckWrapper) map[string]bool {
	nodeTypeSlice := check.NodeType
	// simple hack to get the count to match for empty node type
	// TODO: Modify this when a new plugin of Job type is created
	if len(nodeTypeSlice) == 1 && nodeTypeSlice[0] == NodeTypeNone {
		nodeTypeSlice = []NodeType{NodeTypeMaster}
	}
	nodes := map[string]bool{}
	for _, t := range nodeTypeSlice {
		for _, v := range s.fullReport.Nodes[t] {
			nodes[v] = true
		}
	}
	return nodes
}

func (s *Summarizer) getMissingNodesMapOfCheckWrapper(check *CheckWrapper, nodes map[string]bool) []string {
	allNodes := map[string]bool{}
	for _, nodeType := range check.NodeType {
		for _, v := range s.fullReport.Nodes[nodeType] {
			allNodes[v] = true
		}
	}
	for n := range nodes {
		delete(allNodes, n)
	}
	logrus.Debugf("ID: %v, missing nodes: %v", check.ID, allNodes)
	var missingNodes []string
	for k := range allNodes {
		missingNodes = append(missingNodes, k)
	}
	return missingNodes
}

// Logic:
// - If a check has a non-PASS state on any host, the check is considered mixed.
//   Nodes will list the ones where the check has failed.
// - If a check has all pass, then nodes is empty. All nodes in that host type have passed.
// - If a check has all fail, then nodes is empty. All nodes in that host type have failed.
// - If a check is skipped, then nodes is empty.
func (s *Summarizer) runFinalPassOnCheckWrapper(cw *CheckWrapper) {
	//copy over the actual result info of the test after running the scan
	s.copyDataFromResults(cw)
	nodesMap := s.getNodesMapOfCheckWrapper(cw)
	nodeCount := len(nodesMap)
	logrus.Debugf("id: %s nodeCount: %d", cw.ID, nodeCount)
	if len(cw.Result) == 1 {
		if _, ok := cw.Result[NA]; ok {
			cw.State = NotApplicable
			s.fullReport.NotApplicable++
			return
		}
		if _, ok := cw.Result[kb.FAIL]; ok {
			if len(cw.Result[kb.FAIL]) == nodeCount {
				cw.State = Fail
				s.fullReport.Fail++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[kb.FAIL])
			}
			return
		}
		if _, ok := cw.Result[kb.PASS]; ok {
			if len(cw.Result[kb.PASS]) == nodeCount {
				cw.State = Pass
				s.fullReport.Pass++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[kb.PASS])
			}
			return
		}
		if _, ok := cw.Result[SKIP]; ok {
			if len(cw.Result[SKIP]) == nodeCount {
				cw.State = Skip
				s.fullReport.Skip++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[SKIP])
			}
			return
		}
		if _, ok := cw.Result[kb.WARN]; ok {
			if len(cw.Result[kb.WARN]) == nodeCount {
				cw.State = Warn
				s.fullReport.Warn++
			} else {
				cw.State = Mixed
				s.fullReport.Warn++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[kb.WARN])
			}
			return
		}
		for k := range cw.Result {
			if len(cw.Result[k]) == nodeCount {
				cw.State = Fail
				s.fullReport.Fail++
				cw.Result[k] = nil
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[k])
			}
		}
		return
	}
	s.fullReport.Fail++
	cw.State = Mixed
	for k := range cw.Result {
		if k == kb.PASS {
			continue
		}
		for n := range cw.Result[k] {
			cw.Nodes = append(cw.Nodes, n)
		}
	}
}

func (s *Summarizer) copyDataFromResults(cw *CheckWrapper) {
	checkFromResults := s.checkWrappersMaps[cw.ID]
	if checkFromResults == nil {
		return
	}
	cw.Audit = checkFromResults.Audit
	cw.AuditConfig = checkFromResults.AuditConfig
	cw.ActualValueNodeMap = checkFromResults.ActualValueNodeMap
	cw.ExpectedResult = checkFromResults.ExpectedResult
	cw.Remediation = checkFromResults.Remediation
	cw.TestInfo = checkFromResults.TestInfo
}

func (s *Summarizer) runFinalPass() error {
	logrus.Debugf("running final pass")
	s.fullReport.Version = s.BenchmarkVersion
	groups := s.fullReport.GroupWrappers
	for _, group := range groups {
		for _, cw := range group.CheckWrappers {
			logrus.Debugf("before final pass on check")
			printCheckWrapper(cw)
			s.runFinalPassOnCheckWrapper(cw)
			logrus.Debugf("after final pass on check")
			printCheckWrapper(cw)
		}
	}

	return nil
}

func (s *Summarizer) Summarize() error {
	logrus.Infof("summarize")

	// Walk through the host folders
	hostsDir, err := ioutil.ReadDir(s.InputDirectory)
	if err != nil {
		return fmt.Errorf("error listing directory: %v", err)
	}

	for _, hostDir := range hostsDir {
		if !hostDir.IsDir() {
			continue
		}
		hostname := hostDir.Name()
		logrus.Debugf("hostDir: %s", hostname)

		// Check for errors before proceeding
		errorLogFile := fmt.Sprintf("%s/%s/%s", s.InputDirectory, hostname, DefaultErrorLogFileName)
		if _, err := os.Stat(errorLogFile); err == nil {
			data, err := ioutil.ReadFile(errorLogFile)
			if err != nil {
				return fmt.Errorf("error reading file %v: %v", errorLogFile, err)
			}
			// error.log file gets created due to redirection, hence check if not empty
			if len(data) > 0 {
				logrus.Infof("found error file")
				return fmt.Errorf("%v", string(data))
			}
			logrus.Infof("found empty error log file: %v for host: %v, ignoring", DefaultErrorLogFileName, hostname)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("unexpected error finding file %v: %v", errorLogFile, err)
		}

		if err := s.summarizeForHost(hostname); err != nil {
			return fmt.Errorf("error summarizeForHost %v: %v", hostname, err)
		}
	}

	logrus.Debugf("--- before final pass")
	_ = s.printReport()
	if err := s.runFinalPass(); err != nil {
		return fmt.Errorf("error running final pass on the report: %v", err)
	}
	logrus.Debugf("--- after final pass")
	_ = s.printReport()
	return s.save()
}

func (s *Summarizer) printReport() error {
	logrus.Debugf("printing report")

	for _, gw := range s.fullReport.GroupWrappers {
		for _, cw := range gw.CheckWrappers {
			printCheckWrapper(cw)
		}
	}

	b, err := json.MarshalIndent(s.fullReport, "", " ")
	if err != nil {
		return fmt.Errorf("error marshalling report: %s", err.Error())
	}

	logrus.Debugf("json txt: %s", b)
	return nil
}

func printCheck(check *kb.Check) {
	logrus.Debugf("KB check: %+v", check)
}

func printCheckWrapper(cw *CheckWrapper) {
	logrus.Debugf("checkWrapper: %+v", cw)
}
