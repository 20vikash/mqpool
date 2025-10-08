package mqpool

func (r *RetryConfig) RetryInit() {
	/*
	   RetryInit() initializes retry queue for r.MainQueue. It will create a new retry queue if
	   r.Auto = true, otherwise uses r.RetryQueueName
	*/
}
