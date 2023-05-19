package discovery

import "testing"

func Test_parseServiceKey(t *testing.T) {
	type args struct {
		serviceKey string
	}
	tests := []struct {
		name            string
		args            args
		wantServiceName string
		wantHttpAddr    string
		wantInstanceId  string
	}{
		{
			"name1",
			args{
				"b20.service.core/76da-blocface-core.blocface-core-headless.gzmfeat3.svc.cluster.local:8001",
			},
			"b20.service.core",
			"blocface-core.blocface-core-headless.gzmfeat3.svc.cluster.local:8001",
			"76da",
		},
		{
			"name2",
			args{
				"b20.service.core/08c3-blocface-core-0.blocface-core-headless.gzmfeat3.svc.cluster.local:8001/7350018126459689285",
			},
			"b20.service.core",
			"blocface-core-0.blocface-core-headless.gzmfeat3.svc.cluster.local:8001",
			"08c3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotServiceName, gotHttpAddr, gotInstanceId := parseServiceKey(tt.args.serviceKey)
			if gotServiceName != tt.wantServiceName {
				t.Errorf("parseServiceKey() gotServiceName = %v, want %v", gotServiceName, tt.wantServiceName)
			}
			if gotHttpAddr != tt.wantHttpAddr {
				t.Errorf("parseServiceKey() gotHttpAddr = %v, want %v", gotHttpAddr, tt.wantHttpAddr)
			}
			if gotInstanceId != tt.wantInstanceId {
				t.Errorf("parseServiceKey() gotInstanceId = %v, want %v", gotInstanceId, tt.wantInstanceId)
			}
		})
	}
}
