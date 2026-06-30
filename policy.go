package config

import "time"

// PolicyType represents the scheduling algorithm to use
type PolicyType string

const (
	PolicyBinPacking    PolicyType = "bin-packing"
	PolicyLoadBalancing PolicyType = "load-balancing"
	PolicyAffinity      PolicyType = "affinity"
	PolicyHybrid        PolicyType = "hybrid"
)

// Config holds all scheduler configuration
type Config struct {
	SchedulerName string
	Policy        PolicyType

	// Bin Packing settings
	BinPacking BinPackingConfig

	// Load Balancing settings
	LoadBalancing LoadBalancingConfig

	// Affinity settings
	Affinity AffinityConfig

	// General
	SchedulingInterval time.Duration
	MaxRetries         int
	QueueSize          int
}

type BinPackingConfig struct {
	// Weight for CPU utilization in scoring (0.0 - 1.0)
	CPUWeight float64
	// Weight for memory utilization in scoring (0.0 - 1.0)
	MemoryWeight float64
	// Target utilization threshold before moving to next node (0.0 - 1.0)
	TargetUtilization float64
	// Prioritize nodes with least remaining capacity (true = pack tightly)
	FirstFitDecreasing bool
}

type LoadBalancingConfig struct {
	// Weight for CPU utilization in scoring
	CPUWeight float64
	// Weight for memory utilization in scoring
	MemoryWeight float64
	// Weight for pod count in scoring
	PodCountWeight float64
	// Maximum pods per node before penalizing
	MaxPodsPerNode int
}

type AffinityConfig struct {
	// Hard affinity rules weight
	RequiredWeight float64
	// Soft affinity preference weight
	PreferredWeight float64
	// Topology spread weight (for zone/rack distribution)
	TopologyWeight float64
	// Default topology keys to consider
	TopologyKeys []string
}

// DefaultConfig returns production-ready defaults
func DefaultConfig() *Config {
	return &Config{
		SchedulerName:      "custom-scheduler",
		Policy:             PolicyBinPacking,
		SchedulingInterval: 100 * time.Millisecond,
		MaxRetries:         3,
		QueueSize:          1000,
		BinPacking: BinPackingConfig{
			CPUWeight:          0.6,
			MemoryWeight:       0.4,
			TargetUtilization:  0.85,
			FirstFitDecreasing: true,
		},
		LoadBalancing: LoadBalancingConfig{
			CPUWeight:      0.4,
			MemoryWeight:   0.3,
			PodCountWeight: 0.3,
			MaxPodsPerNode: 110,
		},
		Affinity: AffinityConfig{
			RequiredWeight:  1.0,
			PreferredWeight: 0.5,
			TopologyWeight:  0.3,
			TopologyKeys:    []string{"kubernetes.io/hostname", "topology.kubernetes.io/zone"},
		},
	}
}
