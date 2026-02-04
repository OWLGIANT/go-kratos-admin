package config

var REDIS_ADDR = ""
var REDIS_PWD = ""

var INFLUX_ADDR = ""
var INFLUX_PWD = ""
var INFLUX_USER = ""

func Set(redisAddr, redisPwd string, influxAddr, influxUser, influxPwd string) {
	REDIS_ADDR = redisAddr
	REDIS_PWD = redisPwd
	INFLUX_ADDR = influxAddr
	INFLUX_PWD = influxPwd
	INFLUX_USER = influxUser
}
