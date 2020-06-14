package workerpool

//WorkerPoolConf 配置
type WorkerPoolConf struct {
	MinCnt        int32 `json:"min_cnt"`
	MaxCnt        int32 `json:"max_cnt"`
	CleanInterval int32 `json:"clean_interval"`
}
