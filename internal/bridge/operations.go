package bridge

// Operation names (dot-namespaced).
const (
	OpTaskList        = "task.list"
	OpTaskGet         = "task.get"
	OpTaskCreate      = "task.create"
	OpTaskUpdate      = "task.update"
	OpTaskComplete    = "task.complete"
	OpTaskUncomplete  = "task.uncomplete"
	OpTaskStart       = "task.start"
	OpTaskStopCurrent = "task.stop_current"
	OpTaskGetCurrent  = "task.get_current"
	OpTaskSetCurrent  = "task.set_current"
	OpTaskArchive     = "task.archive"
	OpTaskRestore     = "task.restore"
	OpProjectList     = "project.list"
	OpTagList         = "tag.list"
	OpStatusGet       = "status.get"
	OpBridgeHealth    = "bridge.health"
)
