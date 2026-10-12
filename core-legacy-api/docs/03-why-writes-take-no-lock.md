# Why Writes Take No Lock

## Overview

Unlike the Config Server implementation, `core-legacy-api` does **not** acquire an application-level lock for write operations.

## Why No Lock Is Needed

Consul KV serializes individual write operations, so concurrent writes cannot produce corrupted or partially merged values. If two requests write different values to the same key at the same time, the resulting value is simply one of the two writes.

## When a Lock Would Be Required

An application-level lock is only needed for a **read-modify-write** operation, where the application:

1. Reads the current state.
2. Makes a decision based on that state.
3. Writes the result.

Without a lock, another write could change the state between the read and the write, invalidating the decision.

## Why the Previous Implementation Needed One

The Config Server implementation had to validate conflicting property keys before adding a property. That validation was a read-modify-write sequence:

1. **Read** the existing keys.
2. **Check** whether the new key conflicts with any of them.
3. **Write** the new property if there is no conflict.

A lock was required to keep the set of keys from changing between the check and the write.

## Why It Is No Longer Required

`core-legacy-api` allows conflicting keys to be stored (see [Conflicting Property Keys](#conflicting-property-keys)). Since the conflict validation is no longer performed, there is no read-modify-write sequence, and therefore no need for a lock.
