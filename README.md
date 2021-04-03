# load-balancer
This repository stores a load balancer prototype, implementing three distinguished processes:

- **balancer:** The load balancer itself, that receives incoming requests from clients and distributes it accross the available nodes.

- **client:** A simple cmdli client, that parses ```load-maxExecTime``` messages from stdin, marshals them on [protobuffs](https://developers.google.com/protocol-buffers), and forwards to the balancer through a tcp socket.

- **server:** The passive server node that simply receives load requests from the balancer and applies them to its current state up to ```maxExecTime``` seconds. Periodically sends a heartbeat message to the **balancer**, informing its current load and availability.


## Load distribution algorithm
The load distribution to server nodes follows a simple algorithm, that acts like a roulette where each server odds considers its current load.

1. Sorts the list of the *N* online nodes on ascending order based on their current load;

    [10, 10, 20, 40, 50, 50, 70]

2. Retrieves the first ```ceil(N/2)``` nodes from that list;

    [10, 10, 20, 40]

3. Create a list of its complements, the available load on each of them;

    [90, 90, 80, 60] --- total: 320

4. Calculate the percentage of each available load based on the list sum;

    [28.125, 28.125, 25, 18.75]

5. Sum each percentage, creating roulette intervals;

    [28.125, 56.25, 81.25, 100]

6. Draws a rand num from within [1, 100] range, and check on which roulette interval *i* it falls off;

7. Send the load request to the *i* server from the node list retrieved on step 2.

**OBS:** If during the iteration of step 3, a *n* node doesn't have the necessary available load to handle the current request, the *n* node and all nodes of index *m*, such as *m* > *n*, are discarded. If no node is capable to handle, the message is ignored.


## Usage
1. After compiling all different programs, run the load balancer as simply:
    ```
    ./balancer/balancer
    ```

2. The load balancer will be listening for node join requests on ```:9000``` port. Now run the desirable number of server processes as:
    ```
    ./server/server -listen 127.0.0.1:9001 -join 127.0.0.1:9000
    ./server/server -listen 127.0.0.1:9002 -join 127.0.0.1:9000
    ./server/server -listen 127.0.0.1:9003 -join 127.0.0.1:9000
    ```

3. Run the client process and start sending ```load-maxExecTime``` from stdin. You can both manually send text requests or forward a workload following [the example provided](client/example.load). But keep in mind that running with the later may incur in **messages being discarded due to the lack of resources**, because a batch of messages will be forwarded to nodes at once, before their current load (*i.e.* considering a subset of actually applied messages) is updated by a heartbeat.
    ```
    ./client/client -send 127.0.0.1:8000
    ./client/client -send 127.0.0.1:8000 < example.load
    ```


## Known limitations
- Avoid sending the value ```10``` on load requests. It translates to the same byte enconding of ```\n```, the chosen delimiter for the protobuff stream. Sending it won't crash the entire process, but the message will be ignored.

- Right now, messages unhandled because of the lack of available resources or offline nodes are simply discarded. It wasn't a pre-requisite for this project to assure the process of all messages. The enqueue of those messages could be later implemented by an extra channel, being priorly consumed before ```lb.Incoming```.
