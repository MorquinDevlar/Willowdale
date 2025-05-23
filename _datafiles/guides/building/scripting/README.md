# Scripting Language

All scripting is in [ECMAScript 5.1](https://en.wikipedia.org/wiki/ECMAScript) (AKA javascript).

# Room Scripting
See [Room Scripting](SCRIPTING_ROOMS.md)

# Mob Scripting
See [Mob Scripting](SCRIPTING_MOBS.md)

# Item Scripting
See [Item Scripting](SCRIPTING_ITEMS.md)

# Buff Scripting
See [Buff Scripting](SCRIPTING_BUFFS.md)

# Spell Scripting
See [Spell Scripting](SCRIPTING_SPELLS.md)

# Script Functions

[ActorObject Functions](FUNCTIONS_ACTORS.md) - Functions that query or alter user/mob data.

[RoomObject Functions](FUNCTIONS_ROOMS.md) - Functions that query or alter room data.

[ItemObject Functions](FUNCTIONS_ITEMS.md) - Functions that query or alter item data.

[Utility Functions](FUNCTIONS_UTIL.md) - Helper and info functions.

[Messaging Functions](FUNCTIONS_MESSAGING.md) - Helper and info functions.

# Time Periods

Whenever you need to specific a "period" of time, it takes the following string format:

- Should be in the format of: `{num} {unit}` or `{num} real {unit}`
- Unit can be: `rounds`, `hours`, `days`, `weeks`, `months`, `years`
- Default is in-game time, not real time. 
- **To use real time, use the following format:** `{num} real {unit}` - Example: `1 real day`

# Special symbols in user or mob commands:

There are some special prefixes that can help target more specifically than just a name.
These are particularly helpful when there may be other matching targets on a given name:
* `goblin` peaceful vs `goblin` that hit you `(that is fighting), 
* `dagger` vs a `dagger` with enhancements
* user `sam` vs user `samuel`, when sam has already left the room.

These are only useful in `commands` such as `look`, `give`, `attack`, etc.

* `!{number}` - denotes a specific `ItemId` as a target. 
  * `drop !123` will drop `ItemId`=`123`
  * `give !123 to samuel` will give `ItemId`=`123` to a user or mob in the room named `samuel`
* `#{number}` - denotes a specific `Mob Instance Id` as a target.
  * `kick #98` will kick `Mob Instance Id`=`98`
* `@{number}` - denotes a specific `UserId` as a target.
  * `give !123 to @5` will give `ItemId`=`123` to `UserId`=`5`
  * `give !123 to #98` will give `ItemId`=`123` to `Mob Instance Id`=`98`

These are optional, everything can still be referred to by `name` or augmented name ( `dagger#2` etc. )
