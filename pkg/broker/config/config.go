package config

import "fmt"

func BrokerKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
