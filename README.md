# Circuit Breaker

`circuitbreaker` is to Lightning what firewalls are to the internet.

It allows nodes to protect themselves from being flooded with htlcs. With
`circuitbreaker` a maximum to the number of in-flight htlcS can be set on a
per-peer basis. Known and trusted peers for example can be assigned a higher
maximum, while a new channel from a previously unseen node may be limited to
only a few pending htlcs.

Why are limits needed?

In today's Lighting Network payments are routed via a series of hops. Each of
those hops will incur a cost for forwarding that payment. While the htlc of an
hop is in-flight, the associated amount is locked in the hop's outgoing channel.
Those funds cannot be used for another purpose. This can be considered to be an
opportunity cost.

Furthermore each channel has a limited number of htlc 'slots'. The current
maximum is 483 slots. This means that regardless of channel capacity, there can
never be more than 483 htlcs pending. With large channels in particular, it can
happen that all slots are occupied while only a fraction of the channel capacity
is used. In that case the whole channel is considered to be locked. The duration
of the lock can vary from a few seconds to as long as 2 weeks or even more.

When the payment is completed successfully, each hop will collect a routing fee.
But depending on the length of the lock and the htlc amounts, this may be far
from sufficient to cover the costs.

This is where `circuitbreaker` comes in. It puts up a defense around that
valuable channel liquidity and helps to keep the locked coins at work to
maximize routing revenue.

## How to use

### Requirements
* `lnd` version 0.11.0-beta or above.

### Configuration
`circuitbreaker` by default reads its configuration from `~/.circuitbreaker/circuitbreaker.yaml`. 

Below is an example of a configuration that limits the number of pending htlcs
to five by default. For two peers, the limit is lowered to two. A last peer is
allowed to have up to a hundred htlcs in-flight.
```
maxPendingHtlcs: 5

groups:
  - maxPendingHtlcs: 2
    peers:
    - 033220600ae3949f40739955948ca43fc60174c9c51fb51e6debfc27091e58cebe
    - 021561e3cf45345052912c88b0df7deb7c2ec4a1cf08333edb1ed8dbb3fd203d77
  - maxPendingHtlcs: 100
    peers:
    - 02674dabd68df75f78b6b6dc35dd49dd70db5293ca7a68f9cafa76adafabd5dc7c
```

### Run

* Clone this repository
* `go install`
* Execute `circuitbreaker` with the correct command line flags to connect to
  `lnd`. See `circuitbreaker --help` for details.

## Limitations
* This software is alpha quality. Use at your own risk and be careful in particular on mainnet.
* The interfaces on `lnd` aren't optimized for this purpose. Therefore the use
  of a combination of different endpoints is required. This may lead to certain
  corner cases.
* `circuitbreaker` is currently unaware of htlcs that are already in flight when
  it is started.