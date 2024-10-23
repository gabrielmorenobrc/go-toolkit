## Tests

### Unicast


### Broadcast

#### Build and deploy

diego on emtech007 in ~/sdmc/go/toolkit/propagate/experimental
$ go build; for i in {0..19} ; do scp experimental  diegoPropagation-$i-core:~/propagate; done


### Start experimental on all nodes

diego on emtech007 in ~/sdmc/go/toolkit/propagate/experimental
$ for i in {0..19}; do ssh diegoPropagation-$i-core "cd ~/propagate; ./experimental &" > $i-core.log &  done


### Kill application on all nodes

diego on emtech007 in ~/sdmc/go/toolkit/propagate/experimental
$ for i in {0..19}; do ssh diegoPropagation-$i-core "killall experimental"; done;


### Run a test
on diegoPropagation-0-core (for example)
$ curl -s -X POST http://localhost:8000/sendTestPerformanceBroadcast -H "Content-Type: application/json" -d '{  "testId":3}'

$ curl -s -X POST http://localhost:8000/sendTestPerformanceBroadcastLT4k

$ curl -s -X POST http://localhost:8000/sendTestPerformanceBroadcast40k

