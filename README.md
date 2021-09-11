# Circuit Breaker

`circuitbreaker` is to Lightning what firewalls are to the internet.

It allows nodes to protect themselves from being flooded with htlcs. With
`circuitbreaker` a maximum to the number of in-flight htlcS can be set on a
per-peer basis. Known and trusted peers for example can be assigned a higher
maximum, while a new channel from a previously unseen node may be limited to
only a few pending htlcs.

## Why are limits needed?

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

## Hold fees

An alternative to lowering limits is to charge peers for the actual costs that
they generate in both the success and failure cases. For more information about
this idea, see thread [Hold fees: 402 Payment Required for Lightning
itself](https://lists.linuxfoundation.org/pipermail/lightning-dev/2020-October/002826.html)
on the `lightning-dev` mailing list.

Circuit Breaker does not support 'breaking the circuits' when hold fees aren't
paid, but this is a potential direction for the future. It would roughly entail
requiring peers to deposit money for hold fees and blocking forwards once the
peer's balance is zero.

What is currently implemented is only the reporting of these (virtual) hold
fees. A fee schedule can be defined (see configuration below) and the hold fee
that _could have been charged_ is logged for every forward. Additionally a
periodic report is printed that contains the hold fees charged to peers during
the reporting period. Peers that did not offer any htlcs in that period will be
omitted.

When `reportingInterval` is not set, no hold fee reporting will take place.

```log
2020-10-17T20:45:15.708+0200	INFO	Forwarding htlc	{"channel": 39778131669745664, "htlc": 52, "peer_alias": "tester", "peer": "03afe7da13950201562df3fdd6c8b209aab248daee82d773b9dadebba3eeecbb4c", "pending_htlcs": 1, "max_pending_htlcs": 5}
2020-10-17T20:45:15.852+0200	INFO	Resolving htlc	{"channel": 39778131669745664, "htlc": 52, "peer_alias": "tester", "peer": "03afe7da13950201562df3fdd6c8b209aab248daee82d773b9dadebba3eeecbb4c", "pending_htlcs": 0, "hold_time": "143.396033ms", "hold_fee_msat": 4}
2020-10-17T20:45:20.000+0200	INFO	Hold fees report	{"next_report_time": "2020-10-17T20:45:25.000+0200"}
2020-10-17T20:45:20.000+0200	INFO	Report	{"peer_alias": "tester", "peer": "03afe7da13950201562df3fdd6c8b209aab248daee82d773b9dadebba3eeecbb4c", "total_fees_msat": 74, "interval_fees_msat": 4}
```

## How to use

### Requirements
* `go` 1.13
* `lnd` version 0.11.0-beta or above.

### Configuration
`circuitbreaker` by default reads its configuration from `~/.circuitbreaker/circuitbreaker.yaml`. 

This is an example configuration:

```yaml
# Default max pendings htlcs. Limit the number of pending htlcs to five by
# default.
maxPendingHtlcs: 5

# Define exception groups. For two peers, the limit is lowered to two. A last
# peer is allowed to have up to a hundred htlcs in-flight.
groups:
  - maxPendingHtlcs: 2
    peers:
    - 033220600ae3949f40739955948ca43fc60174c9c51fb51e6debfc27091e58cebe
    - 033220600ae3949f40739955948ca43fc60174c9c51fb51e6debfc27091e58ceba
  - maxPendingHtlcs: 3
    peers:
    - 033220600ae3949f40739955948ca43fc60174c9c51fb51e6debfc27091e58cebf

# Define hold fee so that a fully locked 1 BTC channel would yield ~8.5%
# annually.
#
# The hold fee is not actually paid by channel peers and currently only exists
# to raise awareness of the costs of long-holding htlcs.
holdFee:
  # Set the base hold fee to 1 sat per hour to compensate for the usage of an
  # htlc slot. If an imaginary channel of 1 BTC would have all of its 483 slots
  # occupied for a full year, the total hold fee would be 24 * 365 * 483 =
  # 4231080 sats. This translates to a yearly return on the staked bitcoin of
  # ~4.2%.
  baseSatPerHr: 1

  # Set the hold fee rate to 5 parts per million. If an imaginary channel of 1
  # BTC would have all of its funds time-locked for a full year, the total hold
  # fee would be 24 * 365 * 100000000 * 5 / 1000000 = 4380000. This translates
  # to a yearly return on the staked bitcoin of ~4.4%.
  ratePpmPerHr: 5

  # Report (virtually) collected hold fees once per hour.
  reportingInterval: 1h
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
