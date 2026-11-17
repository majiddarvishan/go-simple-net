package neta

import "runtime"

func runtimeGOMAXPROCSImpl() int { return runtime.GOMAXPROCS(0) }
