# Realtime

MiraBase realtime subscriptions deliver authorized record changes to connected
clients. A subscription is evaluated under the subscriber's current collection
rules; a record that the subscriber cannot view is not delivered as ordinary
record data.

Clients subscribe to collections or individual records, receive create, update,
and delete events, and reconnect with their subscription state after network
interruptions. Server-side authorization is checked when subscriptions are
created and when rule-sensitive changes are delivered according to the runtime
policy.

MiraBase also provides live queries over published live plans. Live-query
streams deliver snapshots and keyed deltas, reset when their context can no
longer be replayed safely, and terminate when cancelled or when authorization
is withdrawn.
