// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redisreceiver

import (
	"errors"
	"fmt"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// A map of the INFO data returned from Redis.
type info map[string]string

// Builds protobuf metrics from the combination of a metrics map
// (INFO from Redis) and redisMetrics (built at startup). These are the fixed,
// non keyspace metrics. Returns a list of parsing errors, which can be treated
// like warnings.
func (i info) buildFixedProtoMetrics(metrics []*redisMetric, t *timeBundle) (
	protoMetrics []*metricspb.Metric,
	warnings []error,
) {
	for _, redisMetric := range metrics {
		strVal, ok := i[redisMetric.key]
		if !ok {
			warnings = append(warnings, fmt.Errorf("info key not found: %v", redisMetric.key))
			continue
		}
		if strVal == "" {
			continue
		}
		protoMetric, parsingError := redisMetric.parseMetric(strVal, t)
		if parsingError != nil {
			warnings = append(warnings, parsingError)
			continue
		}
		protoMetrics = append(protoMetrics, protoMetric)
	}
	return protoMetrics, warnings
}

// Builds proto metrics from any 'keyspace' metrics in Redis INFO:
// e.g. "db0:keys=1,expires=2, avg_ttl=3". Returns proto metrics and parsing
// errors, to be treated as warnings, if there were any.
func (i info) buildKeyspaceProtoMetrics(t *timeBundle) (
	protoMetrics []*metricspb.Metric,
	warnings []error,
) {
	const RedisMaxDbs = 16
	for db := 0; db < RedisMaxDbs; db++ {
		key := "db" + strconv.Itoa(db)
		str, ok := i[key]
		if !ok {
			break
		}
		keyspace, parsingError := parseKeyspaceString(db, str)
		if parsingError != nil {
			warnings = append(warnings, parsingError)
			continue
		}
		keyspaceMetrics := buildKeyspaceTriplet(keyspace, t)
		protoMetrics = append(protoMetrics, keyspaceMetrics...)
	}
	return protoMetrics, warnings
}

func (i info) getUptimeInSeconds() (int, error) {
	const uptimeKey = "uptime_in_seconds"
	uptimeStr, ok := i[uptimeKey]
	if !ok {
		return 0, errors.New(uptimeKey + " missing from redis info")
	}
	return strconv.Atoi(uptimeStr)
}
