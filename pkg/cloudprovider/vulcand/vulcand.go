/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package vulcand

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider"
	"code.google.com/p/gcfg"

	"github.com/mailgun/vulcand/engine/etcdng"
	"github.com/mailgun/vulcand/engine"
)

const ProviderName = "vulcand"
const VulcandEtcdServerAddr = "127.0.0.1:2379"


// VulcandBalancer is a fake storage of balancer information
type VulcandBalancer struct {
	Name       string
	Region     string
	ExternalIP net.IP
	Ports      []*api.ServicePort
	Hosts      []string
}

type VulcandUpdateBalancerCall struct {
	Name   string
	Region string
	Hosts  []string
}

// VulcandCloud is a test-double implementation of Interface, TCPLoadBalancer, Instances, and Routes. It is useful for testing.
type VulcandCloud struct {
	ng			  engine.Engine
	Exists        bool
	Err           error
	Calls         []string
	Addresses     []api.NodeAddress
	ExtID         map[string]string
	Machines      []string
	NodeResources *api.NodeResources
	ClusterList   []string
	MasterName    string
	ExternalIP    net.IP
	Balancers     []VulcandBalancer
	UpdateCalls   []VulcandUpdateBalancerCall
	RouteMap      map[string]*FakeRoute
	Lock          sync.Mutex
	cloudprovider.Zone
}


type Config struct {
	EtcdNodes []string
	EtcdKey	  string
}



func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return newVulcandCloud(cfg)
	})
}

func readConfig(config io.Reader) (Config, error) {
	var Config cfg
	if config != nil {
		err = gcfg.ReadInto(&cfg, config)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	cfg.EtcdNodes = []string{VulcandEtcdServerAddr}
	cfg.EtcdKey = "vulcand"
	return cfg, nil
}

func newVulcandCloud(config Config) (*VulcandCloud, error) {

	ng, err := etcdng.New(config.EtcdNodes, config.EtcdKey, nil,
		etcdng.Options{})

	if err != nil {
		return nil, err
	}

	vc := &VulcandCloud {
		ng: ng
    }

	return vc, err
}



// GetTCPLoadBalancer is a stub implementation of TCPLoadBalancer.GetTCPLoadBalancer.
func (f *VulcandCloud) GetTCPLoadBalancer(name, region string) (*api.LoadBalancerStatus, bool, error) {


	status := &api.LoadBalancerStatus{}
	status.Ingress = []api.LoadBalancerIngress{{IP: f.ExternalIP.String()}}

	return status, f.Exists, f.Err
}

// CreateTCPLoadBalancer is a test-spy implementation of TCPLoadBalancer.CreateTCPLoadBalancer.
// It adds an entry "create" into the internal method call record.
func (f *VulcandCloud) CreateTCPLoadBalancer(name, region string, externalIP net.IP, ports []*api.ServicePort, hosts []string, affinityType api.ServiceAffinity) (*api.LoadBalancerStatus, error) {
	//f.addCall("create")
	f.Balancers = append(f.Balancers, VulcandBalancer{name, region, externalIP, ports, hosts})

	status := &api.LoadBalancerStatus{}
	status.Ingress = []api.LoadBalancerIngress{{IP: f.ExternalIP.String()}}

	return status, f.Err
}

// UpdateTCPLoadBalancer is a test-spy implementation of TCPLoadBalancer.UpdateTCPLoadBalancer.
// It adds an entry "update" into the internal method call record.
func (f *VulcandCloud) UpdateTCPLoadBalancer(name, region string, hosts []string) error {
	f.addCall("update")
	f.UpdateCalls = append(f.UpdateCalls, VulcandUpdateBalancerCall{name, region, hosts})
	return f.Err
}

// EnsureTCPLoadBalancerDeleted is a test-spy implementation of TCPLoadBalancer.EnsureTCPLoadBalancerDeleted.
// It adds an entry "delete" into the internal method call record.
func (f *VulcandCloud) EnsureTCPLoadBalancerDeleted(name, region string) error {
	f.addCall("delete")
	return f.Err
}




type FakeRoute struct {
	ClusterName string
	Route       cloudprovider.Route
}

func (f *VulcandCloud) addCall(desc string) {
	f.Calls = append(f.Calls, desc)
}

// ClearCalls clears internal record of method calls to this VulcandCloud.
func (f *VulcandCloud) ClearCalls() {
	f.Calls = []string{}
}

func (f *VulcandCloud) ListClusters() ([]string, error) {
	return f.ClusterList, f.Err
}

func (f *VulcandCloud) Master(name string) (string, error) {
	return f.MasterName, f.Err
}

func (f *VulcandCloud) Clusters() (cloudprovider.Clusters, bool) {
	return f, false
}

// ProviderName returns the cloud provider ID.
func (f *VulcandCloud) ProviderName() string {
	return ProviderName
}

// TCPLoadBalancer returns a fake implementation of TCPLoadBalancer.
// Actually it just returns f itself.
func (f *VulcandCloud) TCPLoadBalancer() (cloudprovider.TCPLoadBalancer, bool) {
	return f, true
}

// Instances returns a fake implementation of Instances.
//
// Actually it just returns f itself.
func (f *VulcandCloud) Instances() (cloudprovider.Instances, bool) {
	return f, false
}

func (f *VulcandCloud) Zones() (cloudprovider.Zones, bool) {
	return f, false
}

func (f *VulcandCloud) Routes() (cloudprovider.Routes, bool) {
	return f, false
}

func (f *VulcandCloud) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("unimplemented")
}

// Implementation of Instances.CurrentNodeName
func (f *VulcandCloud) CurrentNodeName(hostname string) (string, error) {
	return hostname, nil
}

// NodeAddresses is a test-spy implementation of Instances.NodeAddresses.
// It adds an entry "node-addresses" into the internal method call record.
func (f *VulcandCloud) NodeAddresses(instance string) ([]api.NodeAddress, error) {
	f.addCall("node-addresses")
	return f.Addresses, f.Err
}

// ExternalID is a test-spy implementation of Instances.ExternalID.
// It adds an entry "external-id" into the internal method call record.
// It returns an external id to the mapped instance name, if not found, it will return "ext-{instance}"
func (f *VulcandCloud) ExternalID(instance string) (string, error) {
	f.addCall("external-id")
	return f.ExtID[instance], f.Err
}

// InstanceID returns the cloud provider ID of the specified instance.
func (f *VulcandCloud) InstanceID(instance string) (string, error) {
	f.addCall("instance-id")
	return f.ExtID[instance], nil
}

// List is a test-spy implementation of Instances.List.
// It adds an entry "list" into the internal method call record.
func (f *VulcandCloud) List(filter string) ([]string, error) {
	f.addCall("list")
	result := []string{}
	for _, machine := range f.Machines {
		if match, _ := regexp.MatchString(filter, machine); match {
			result = append(result, machine)
		}
	}
	return result, f.Err
}

func (f *VulcandCloud) GetZone() (cloudprovider.Zone, error) {
	f.addCall("get-zone")
	return f.Zone, f.Err
}

func (f *VulcandCloud) GetNodeResources(name string) (*api.NodeResources, error) {
	f.addCall("get-node-resources")
	return f.NodeResources, f.Err
}

func (f *VulcandCloud) ListRoutes(clusterName string) ([]*cloudprovider.Route, error) {
	f.Lock.Lock()
	defer f.Lock.Unlock()
	f.addCall("list-routes")
	var routes []*cloudprovider.Route
	for _, fakeRoute := range f.RouteMap {
		if clusterName == fakeRoute.ClusterName {
			routeCopy := fakeRoute.Route
			routes = append(routes, &routeCopy)
		}
	}
	return routes, f.Err
}

func (f *VulcandCloud) CreateRoute(clusterName string, nameHint string, route *cloudprovider.Route) error {
	f.Lock.Lock()
	defer f.Lock.Unlock()
	f.addCall("create-route")
	name := clusterName + "-" + nameHint
	if _, exists := f.RouteMap[name]; exists {
		f.Err = fmt.Errorf("route %q already exists", name)
		return f.Err
	}
	fakeRoute := FakeRoute{}
	fakeRoute.Route = *route
	fakeRoute.Route.Name = name
	fakeRoute.ClusterName = clusterName
	f.RouteMap[name] = &fakeRoute
	return nil
}

func (f *VulcandCloud) DeleteRoute(clusterName string, route *cloudprovider.Route) error {
	f.Lock.Lock()
	defer f.Lock.Unlock()
	f.addCall("delete-route")
	name := route.Name
	if _, exists := f.RouteMap[name]; !exists {
		f.Err = fmt.Errorf("no route found with name %q", name)
		return f.Err
	}
	delete(f.RouteMap, name)
	return nil
}
