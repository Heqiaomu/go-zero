package logx

// A LogConf is a logging config.
type LogConf struct {
	ServiceName         string `json:"serviceName,optional"`
	Mode                string `json:"mode,default=console"`
	Encoding            string `json:"encoding,default=json,options=[json,plain]"`
	TimeFormat          string `json:"timeFormat,optional"`
	Path                string `json:"path,default=logs"`
	Level               string `json:"level,default=info,options=[info,error,severe]"`
	Compress            bool   `json:"compress,optional"`
	KeepDays            int    `json:"keepDays,optional"`
	MaxSize             int    `json:"maxSize,optional"`
	MaxBackup           int    `json:"maxBackup,optional"`
	StackCooldownMillis int    `json:"stackCooldownMillis,default=100"`
}
