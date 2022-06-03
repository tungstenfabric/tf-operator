package v1alpha1

const (
	LogLevel                                    string = "debug"
	LogDir                                      string = "/var/log/contrail"
	LogLocal                                    int    = 1
	EncapPriority                               string = "MPLSoUDP,MPLSoGRE,VXLAN"
	DpdkUioDriver                               string = "uio_pci_generic"
	CpuCoreMask                                 string = "0x03"
	DpdkMemPerSocket                            int    = 1024
	DpdkCommandAdditionalArgs                   string = ""
	NicOffloadEnable                            bool   = false
	DistSnatProtoPortList                       string = ""
	FlowExportRate                              int    = 0
	CloudOrchestrator                           string = "kubernetes"
	CloudAdminRole                              string = "admin"
	AaaMode                                     string = "no-auth"
	AuthMode                                    string = "noauth"
	AuthParams                                  string = ""
	SslEnable                                   bool   = true
	SslInsecure                                 bool   = false
	ServerCertfile                              string = "/etc/contrail/ssl/certs/server.pem"
	ServerKeyfile                               string = "/etc/contrail/ssl/private/server-privkey.pem"
	ServerCaCertfile                            string = "/etc/contrail/ssl/certs/ca-cert.pem"
	ServerCaKeyfile                             string = "/etc/contrail/ssl/private/ca-key.pem"
	SelfsignedCertsWithIps                      bool   = true
	ControllerNodes                             string = ""
	AnalyticsAlarmEnable                        bool   = false
	AnalyticsSnmpEnable                         bool   = false
	AnalyticsdbEnable                           bool   = false
	AnalyticsNodes                              string = ""
	AnalyticsdbNodes                            string = ""
	AnalyticsSnmpNodes                          string = ""
	AnalyticsApiPort                            int    = 8081
	AnalyticsApiIntrospectPort                  int    = 8090
	AnalyticsdbPort                             int    = 9160
	AnalyticsdbCqlPort                          int    = 9042
	AnalyticsDataTTL                            int    = 48
	AnalyticsConfigAuditTTL                     int    = 2160
	AnalyticsStatisticsTTL                      int    = 4
	AnalyticsFlowTTL                            int    = 2
	TopologyIntrospectPort                      int    = 5921
	QueryengineIntrospectPort                   int    = 8091
	AnalyticsServers                            string = ""
	AnalyticsdbCqlServers                       string = ""
	AnalyticsApiVip                             string = ""
	AnalyticsAlarmNodes                         string = ""
	AlarmgenIntrospectPort                      int    = 5995
	AlarmgenPartitions                          int    = 30
	AlarmgenRedisAggregateDbOffset              string = "1"
	BgpPort                                     int    = 179
	BgpAutoMesh                                 bool   = true
	BgpEnable4Byte                              bool   = false
	BgpAsn                                      int    = 64512
	CollectorPort                               int    = 8086
	CollectorIntrospectPort                     int    = 8089
	CollectorSyslogPort                         int    = 514
	CollectorSflowPort                          int    = 6343
	CollectorIpfixPort                          int    = 4739
	CollectorProtobufPort                       int    = 3333
	CollectorStructuredSyslogPort               int    = 3514
	SnmpcollectorIntrospectPort                 int    = 5920
	CollectorServers                            string = ""
	CassandraPort                               int    = 9161
	CassandraCqlPort                            int    = 9041
	CassandraSslStoragePort                     int    = 7013
	CassandraStoragePort                        int    = 7012
	CassandraJmxLocalPort                       int    = 7201
	CassandraMinimumDiskGB                      int    = 4
	ConfigNodes                                 string = ""
	ConfigdbNodes                               string = ""
	ConfigApiPort                               int    = 8082
	ConfigApiIntrospectPort                     int    = 8084
	ConfigAPIAdminPort                          int    = 8095
	ConfigdbPort                                int    = 9161
	ConfigdbCqlPort                             int    = 9041
	ConfigServers                               string = ""
	ConfigdbServers                             string = ""
	ConfigdbCqlServers                          string = ""
	ConfigApiVip                                string = ""
	ConfigApiSslEnable                          bool   = true
	ConfigApiServerCertfile                     string = "/etc/contrail/ssl/certs/server.pem"
	ConfigApiServerKeyfile                      string = "/etc/contrail/ssl/private/server-privkey.pem"
	ConfigApiServerCaCertfile                   string = "/etc/contrail/ssl/certs/ca-cert.pem"
	ConfigAPIWorkerCount                        int    = 1
	ConfigSchemaIntrospectPort                  int    = 8087
	ConfigSvcMonitorIntrospectPort              int    = 8088
	ConfigDeviceManagerIntrospectPort           int    = 8096
	CassandraSslEnable                          bool   = true
	CassandraSslCertfile                        string = "/etc/contrail/ssl/certs/server.pem"
	CassandraSslKeyfile                         string = "/etc/contrail/ssl/private/server-privkey.pem"
	CassandraSslCaCertfile                      string = "/etc/contrail/ssl/certs/ca-cert.pem"
	CassandraSslKeystorePassword                string = "astrophytum"
	CassandraSslTruststorePassword              string = "ornatum"
	CassandraSslProtocol                        string = "TLS"
	CassandraSslAlgorithm                       string = "SunX509"
	CassandraSslCipherSuites                    string = "[TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_DHE_RSA_WITH_AES_128_CBC_SHA,TLS_DHE_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA]"
	CassandraConfigMemtableFlushWriter          int    = 4
	CassandraConfigConcurrectCompactors         int    = 4
	CassandraConfigCompactionThroughputMbPerSec int    = 256
	CassandraConfigConcurrectReads              int    = 64
	CassandraConfigConcurrectWrites             int    = 64
	CassandraConfigMemtableAllocationType       string = "offheap_objects"
	CNIConfigPath                               string = "/etc/cni"
	CNIBinaryPath                               string = "/opt/cni/bin"
	ControlNodes                                string = ""
	ControlIntrospectPort                       int    = 8083
	DnsNodes                                    string = ""
	DnsServerPort                               int    = 53
	DnsIntrospectPort                           int    = 8092
	RndcKey                                     string = "xvysmOR8lnUQRBcunkC6vg=="
	UseExternalTFTP                             bool   = false
	ZookeeperNodes                              string = ""
	ZookeeperPort                               int    = 2181
	ZookeeperElectionPort                       int    = 2888
	ZookeeperServerPort                         int    = 3888
	ZookeeperAdminEnableServer                  bool   = true
	ZookeeperAdminPort                          int    = 2182
	ZookeeperPorts                              string = "2888:3888"
	ZookeeperServers                            string = ""
	ZookeeperServersSpaceDelim                  string = ""
	RabbitmqNodes                               string = ""
	RabbitmqErlangCookie                        string = "47EFF3BB-4786-46E0-A5BB-58455B3C2CB4"
	RabbitmqNodePort                            int    = 5673
	RabbitmqErlEpmdPort                         int    = 4371
	RabbitmqServers                             string = ""
	RabbitmqSslCertfile                         string = "/etc/contrail/ssl/certs/server.pem"
	RabbitmqSslKeyfile                          string = "/etc/contrail/ssl/private/server-privkey.pem"
	RabbitmqSslCacertfile                       string = "/etc/contrail/ssl/certs/ca-cert.pem"
	RabbitmqSslFailIfNoPeerCert                 bool   = true
	RabbitmqVhost                               string = "/"
	RabbitmqUser                                string = "guest"
	RabbitmqPassword                            string = "guest"
	RabbitmqUseSsl                              bool   = true
	RabbitmqSslVer                              string = "tlsv1_2"
	RabbitmqClientSslCertfile                   string = "/etc/contrail/ssl/certs/server.pem"
	RabbitmqClientSslKeyfile                    string = "/etc/contrail/ssl/private/server-privkey.pem"
	RabbitmqClientSslCacertfile                 string = "/etc/contrail/ssl/certs/ca-cert.pem"
	RabbitmqHeartbeatInterval                   int    = 10
	RabbitmqMirroredQueueMode                   string = "all"
	RabbitmqClusterPartitionHandling            string = "autoheal"
	RedisPort                                   int    = 6389
	RedisSslEnable                              bool   = true
	RedisSslCertfile                            string = "/etc/contrail/ssl/certs/server.pem"
	RedisSslKeyfile                             string = "/etc/contrail/ssl/private/server-privkey.pem"
	KafkaNodes                                  string = ""
	KafkaPort                                   int    = 9092
	KafkaServers                                string = ""
	KafkaSslEnable                              bool   = true
	KafkaSslCertfile                            string = "/etc/contrail/ssl/certs/server.pem"
	KafkaSslKeyfile                             string = "/etc/contrail/ssl/private/server-privkey.pem"
	KafkaSslCacertfile                          string = "/etc/contrail/ssl/certs/ca-cert.pem"
	KeystoneAuthAdminTenant                     string = "admin"
	KeystoneAuthAdminUser                       string = "admin"
	KeystoneAuthAdminPassword                   string = "contrail123"
	KeystoneAuthUserDomainName                  string = "Default"
	KeystoneAuthProjectDomainName               string = "Default"
	KeystoneAuthRegionName                      string = "RegionOne"
	KeystoneAuthUrlVersion                      string = "/v3"
	KeystoneAuthHost                            string = "127.0.0.1"
	KeystoneAuthProto                           string = "https"
	KeystoneAuthAdminPort                       int    = 35357
	KeystoneAuthPort                            int    = 5000
	KeystoneAuthUrlTokens                       string = "/v3/auth/tokens"
	KeystoneAuthInsecure                        bool   = true
	KeystoneAuthCertfile                        string = ""
	KeystoneAuthKeyfile                         string = ""
	KeystoneAuthCaCertfile                      string = ""
	KubemanagerNodes                            string = ""
	KubernetesApiNodes                          string = ""
	KubernetesApiServer                         string = "10.96.0.1"
	KubernetesApiPort                           int    = 8080
	KubernetesApiSSLPort                        int    = 6443
	KubernetesClusterName                       string = "k8s"
	KubernetesDNSDomainName                     string = "k8s"
	KubernetesPodSubnet                         string = "10.32.0.0/12"
	KubernetesIpFabricSubnets                   string = "10.64.0.0/12"
	KubernetesServiceSubnet                     string = "10.96.0.0/12"
	KubernetesIPFabricForwarding                bool   = false
	KubernetesIPFabricSnat                      bool   = true
	KubernetesPublicFIPPoolTemplate             string = "{'project' : '%s-default', 'domain': 'default-domain', 'name': '__fip_pool_public__' , 'network' : '__public__'}"
	KubernetesHostNetworkService                bool   = false
	KubernetesUseKubeadm                        bool   = true
	KubernetesServiceAccount                    string = ""
	VcenterFabricManagerNodes                   string = ""
	MetadataProxySecret                         string = "contrail"
	BarbicanUser                                string = "barbican"
	BarbicanPassword                            string = "contrail123"
	AgentMode                                   string = "kernel"
	ExternalRouters                             string = ""
	Subcluster                                  string = ""
	VrouterComputeNodeAddress                   string = ""
	VrouterCryptInterface                       string = "crypt0"
	VrouterDecryptInterface                     string = "decrypt0"
	VrouterDecryptKey                           int    = 15
	VrouterModuleOptions                        string = ""
	FabricSnatHashTableSize                     int    = 4096
	TsnEvpnMode                                 bool   = false
	TsnNodes                                    string = "[]"
	PriorityId                                  string = ""
	PriorityBandwidth                           string = ""
	PriorityScheduling                          string = ""
	QosQueueId                                  string = ""
	QosLogicalQueues                            string = ""
	QosDefHwQueue                               bool   = false
	PriorityTagging                             bool   = true
	SloDestination                              string = "collector"
	SampleDestination                           string = "collector"
	WebuiNodes                                  string = ""
	WebuiJobServerPort                          int    = 3000
	KueUiPort                                   int    = 3002
	WebuiHttpListenPort                         int    = 8180
	WebuiHttpsListenPort                        int    = 8143
	WebuiSslKeyFile                             string = "/etc/contrail/webui_ssl/cs-key.pem"
	WebuiSslCertFile                            string = "/etc/contrail/webui_ssl/cs-cert.pem"
	WebuiSslCiphers                             string = "ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:AES256-SHA"
	WebuiStaticAuthUser                         string = "admin"
	WebuiStaticAuthPassword                     string = "contrail123"
	WebuiStaticAuthRole                         string = "cloudAdmin"
	XmppServerPort                              int    = 5269
	XmppSslEnable                               bool   = true
	XmppServerCertfile                          string = "/etc/contrail/ssl/certs/server.pem"
	XmppServerKeyfile                           string = "/etc/contrail/ssl/private/server-privkey.pem"
	XmppServerCaCertfile                        string = "/etc/contrail/ssl/certs/ca-cert.pem"
	LinklocalServicePort                        int    = 80
	LinklocalServiceName                        string = "metadata"
	LinklocalServiceIp                          string = "169.254.169.254"
	IpfabricServicePort                         int    = 8775
	IntrospectSslEnable                         bool   = true
	IntrospectSslInsecure                       bool   = true
	IntrospectCertfile                          string = "/etc/contrail/ssl/certs/server.pem"
	IntrospectKeyfile                           string = "/etc/contrail/ssl/private/server-privkey.pem"
	IntrospectCaCertfile                        string = "/etc/contrail/ssl/certs/ca-cert.pem"
	IntrospectListenAll                         bool   = true
	SandeshSslEnable                            bool   = true
	SandeshCertfile                             string = "/etc/contrail/ssl/certs/server.pem"
	SandeshKeyfile                              string = "/etc/contrail/ssl/private/server-privkey.pem"
	SandeshCaCertfile                           string = "/etc/contrail/ssl/certs/ca-cert.pem"
	MetadataSslEnable                           bool   = false
	MetadataSslCertfile                         string = ""
	MetadataSslKeyfile                          string = ""
	MetadataSslCaCertfile                       string = ""
	MetadataSslCertType                         string = ""
	ConfigureIptables                           string = "false"
	FwaasEnable                                 bool   = false
	TorAgentOvsKa                               int    = 10000
	TorType                                     string = "ovs"
	TorOvsProtocol                              string = "tcp"
	ToragentSslCertfile                         string = "/etc/contrail/ssl/certs/server.pem"
	ToragentSslKeyfile                          string = "/etc/contrail/ssl/private/server-privkey.pem"
	ToragentSslCacertfile                       string = "/etc/contrail/ssl/certs/ca-cert.pem"
	RabbitmqInstance                            string = "rabbitmq1"
	AnalyticsCassandraInstance                  string = "analyticsdb1"
	CassandraInstance                           string = "configdb1"
	ConfigInstance                              string = "config1"
	RedisInstance                               string = "redis1"
	ZookeeperInstance                           string = "zookeeper1"
	AnalyticsInstance                           string = "analytics1"
	AnalyticsAlarmInstance                      string = "analyticsalarm1"
	AnalyticsSnmpInstance                       string = "analyticssnmp1"
	KubemanagerInstance                         string = "kubemanager1"
	WebuiInstance                               string = "webui1"
	OpenShiftClusterName                        string = "test"
	OpenShiftPodSubnet                          string = "10.128.0.0/14"
	OpenShiftServiceSubnet                      string = "172.30.0.0/16"
	OpenShiftDNSDomain                          string = "example.com"
	OpenShiftCNIConfigPath                      string = "/etc/kubernetes/cni"
	OpenShiftCNIBinaryPath                      string = "/var/lib/cni/bin"
)
