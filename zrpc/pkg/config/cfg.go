package config

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/zeromicro/go-zero/zrpc/pkg/etcd"

	"time"

	"io/ioutil"

	log "github.com/Heqiaomu/glog"

	util "github.com/Heqiaomu/goutil"
	"github.com/spf13/viper"
)

const (
	defaultReadMode = "local"
	etcdReadMode    = "etcd"

	defaultEnvPrefix = "APISERVER"

	defaultFileExt  = "toml"
	defaultFilePath = "conf/cfg.toml"

	defaultServiceName            = "other"
	defaultEtcdKeyFormat          = "/%s/config/cfg.toml"
	defaultEtcdConfigType         = "toml"
	defaultEtcdHost               = "127.0.0.1:2379"
	defaultEtcdHostProtocolFormat = "http://%s"
	defaultEtcdTimeout            = 3
)

// ConfigManager define config manager
type ConfigManager struct {
	readMode string // viper read mode e.g. etcd or local
	host     string // etcd host e.g. "127.0.0.1:2379"
	filePath string // etcd configure key e.g. "/b20/config/cfg.toml"
}

// NewConfigManager new config manager
func NewConfigManager(mode, host, name string) *ConfigManager {
	if mode == "" {
		mode = defaultReadMode
	}
	if host == "" {
		host = defaultEtcdHost
	}
	if name == "" {
		name = defaultServiceName
	}

	filePath := fmt.Sprintf(defaultEtcdKeyFormat, name)

	cm := &ConfigManager{
		readMode: mode,
		host:     host,
		filePath: filePath,
	}
	return cm
}

// NewConfig new Configure
func (cm *ConfigManager) NewConfig() error {
	if cm.readMode == etcdReadMode {
		return cm.newRemoteConfig()
	}
	return cm.newLocalConfig()
}

// newLocalConfig return local configure instance
func (cm *ConfigManager) newLocalConfig() (err error) {

	// set local file path
	viper.SetConfigFile(defaultFilePath)
	ext, err := util.Ext(defaultFilePath, viper.SupportedExts...)
	if err != nil {
		fmt.Println(err.Error())
		ext = defaultFileExt
		return
	}
	viper.SetConfigType(ext)

	// read configure from env
	viper.AutomaticEnv()                 // Read Env value
	viper.SetEnvPrefix(defaultEnvPrefix) // Env prefix : APISERVER
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.ReadInConfig()

	// watch config and set onchange function
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {

		// TODO: local file change
		log.Info(fmt.Sprintf("Config file changed: %s\n", e.Name))
	})
	return
}

// newRemoteConfig return remote configure instance
// 1. implement viper etcd v3
// 2. update local config.toml
func (cm *ConfigManager) newRemoteConfig() (err error) {

	err = cm.InitRemoteConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	viper.AddRemoteProvider(cm.readMode, fmt.Sprintf(defaultEtcdHostProtocolFormat, cm.host), cm.filePath)
	viper.SetConfigType(defaultEtcdConfigType)

	// read configure from env
	viper.AutomaticEnv()                 // Read Env value
	viper.SetEnvPrefix(defaultEnvPrefix) // Env prefix : APISERVER
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	// read from remote config the first time.
	err = viper.ReadRemoteConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	we, err := etcd.New([]string{cm.host}, time.Duration(defaultEtcdTimeout)*time.Second)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	_, err = we.WatchKey(cm.filePath, WatchConfig)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	return
}

// WatchConfig watch with etcd v3 api
func WatchConfig(eventType int32, key string, value string, createRevision int64, modRevision int64) interface{} {
	log.Info(fmt.Sprint(eventType, string(key), string(value), createRevision, modRevision))
	err := viper.ReadRemoteConfig()
	if err != nil {
		log.Error(fmt.Sprintf("unable to read remote config: %v", err))
	}

	// async write config file
	go WriteToLocal()

	return nil
}

// WriteToLocal write configure to local file
func WriteToLocal() {
	err := viper.WriteConfigAs(defaultFilePath)
	if err != nil {
		log.Error(fmt.Sprintf("unable to write config to %s error: %v", defaultFilePath, err))
	}
}

// InitRemoteConfig if remote config is null, put local config
func (cm *ConfigManager) InitRemoteConfig() (err error) {
	we, err := etcd.New([]string{cm.host}, time.Duration(defaultEtcdTimeout)*time.Second)
	if err != nil {
		return
	}
	defer we.Close()

	v, err := we.Get(cm.filePath)
	if err != nil {
		return
	}
	if v == "" {
		contents, err := ioutil.ReadFile(defaultFilePath)
		if err != nil {
			return err
		}
		return we.Put(cm.filePath, string(contents))
	}
	return nil
}
