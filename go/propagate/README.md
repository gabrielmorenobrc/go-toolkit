
## Purpose

This is for now mostly a sandbox to challenge ideas and let challenges show by themselves towards config
propagation design phase.
One additional goal here is to see how much can we go far with the less amount of dependencies and complexity,
both in code and infrastructure, yet keep the goals true (overall *use case* latency mostly)
The minimalistic approach in that regard would be use the already enabled http infrastructure (no need to ask for new
ports open, firewalls, etc) for the transport layer and code the "gossip factor" to what we actually need and nothing
but that rather than to accommodate our needs to actual GOSSIP spec and 3rd party libraries.
With all that said, so far, looks like we could get away with something like this...

## Terms & Notes

### Node
The single code artifact the integration code needs to know about. It serves the API for other nodes to connect to and
provides the client API to send messages out to the network

### Primary Peers 
Other nodes to be cofigured as entry points to the network candidates, as opposed to have just one, should some fail.

### Address
By defining the address of a node as the full base URL to its propagation API (IE: http://host:port/basepath) we allow
to run more than one node on the same environment as well as publishing the node API under any arbitrary path

### Transaction id
An identifier for the request or broadcast message created at its originating node and that's meant to be unique within
the network in a given time window.
The transaction id is key to:
. Cut off storming potential by caching transaction ids already processed in the nodes
. Provide idempotency over request/response allowing for additional resilience against links going down during the
response phase and the originator trying an alternative path by caching the generated response on destination linked to
the transaction id.
. For the application layer to send back broadcast messages in response to a particular broadcast message.

### Request
Synchronous request/response message. At a given node the node either provides the response, should be the destination
itself, or attempsts forwards through its known conenctions until it gets a response to return back or fails.

### Broadcast
A message with no specific destination that should be attempted to be sent to all the nodes participating in the network.
Broadcast messaging does not expect a response. We could, however, build a multiple responses wait and collect layer on
top of this.

### Latency and performance 
Should be required we can replace json by a stupidly simple reflection-based binary serialization. 
For the broad/multicast feature we could consider UDP and shouldn't change substantially any logic, but we should really
prove we can't get away with http.
For the sync request/response forwarding Is not clear there will be any major improvements going down networking layers
as the real diferential will be what the business code handlign the packets does.
In any case so far CPU nor http/json transmission overhead seem to be a significant factor towards the 300ms latency goals.
The nature of the different use cases regardless of the transmission technology is likely to dictate the major part of the
latency.

## Optimization candidates and things we probably don't need to do (as in compared to gossip)

### Stormy weather
Optimizing potentially redundant broadcast messages to make things as less "stormy" as possible not fully considered
yet. However, we are not propagating broadcast packets from a given point to another shall the latter be already included
in the trajectory record held by the packet. That itself accounts for a significant part of it. However, not enough for 
scaling. We should be able to easily implement gossip or anything else without really changing anything anywhere else in
the architecture (nor in the rest of the code, really).

### Announce
May be We can get rid of announce cycles by just announcing incoming members and let the new member ask for the current member 
list to its peers. For the sanity check factor, see below.

### Leaves
Letting everyone remove from their members list failed conections just in time should be enough. I don't think we need to
propagate member down messages. We could, however, have a response layer that informs the calling hop of any hop up in the
chain that failed. So we can propagate back the jsut in time cleanup towards re-calculating routes, should we need to
implement routes at all which would only apply to request/response and multicast.

### Multicast
As opposed to broadcast. Is it worth without routes resolution? This could be an "optimization"

### Retries of n-casted messages
Not necessary at all given the current state, period. We are already running on a chain of tcp connections. Not likely, 
statistically, retrying something that already failed at the TCP layer would add up to very much. We could, howver 
propagate errors upon processing an n-cast message from the nodes failing upon actually processing them(exceptions). 
I'd let that to the application layer for now. But we could, rather simple, really. 

