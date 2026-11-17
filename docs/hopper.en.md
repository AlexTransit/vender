# Hoppers
English | [Русский](hopper.ru.md)

- There are two options for execution.

## Hopper

MDB address ```40-78```

- A separate card for each hopper

Each hopper has its own address. Each hopper is an independent device.
The hopper is registered uniquely.

```device "evend.hopper1" { } ```

> It is possible to manage multiple hoppers simultaneously.
> Due to an error in the manufacturer's firmware, it does not determine if the motor is disconnected; it only determines overload by current.

## Multihopper

MDB address ```B8```

- One card for all hoppers.

Registration ```device "evend.multihopper"{ }``` adds 8 hoppers

> It is not possible to manage multiple hoppers simultaneously.

Registration in the configuration:

```yaml
hardware {
    device "evend.hopper1" { }
    device "evend.hopper2" { }
    ...
    device "evend.hopper8" { }
}
```

```yaml
hardware {
    device "evend.multihopper"{ }
}
```

## Commands

### Reset

- Sends a reset command to the device

```evend.multihopper.reset```

```evend.hopper1.reset```

### Run

- Sets the motor operation time. An integer. 1 = 0.1 second of rotation. Allowed values ```0-255```

```evend.multihopper1.run(?)```

```evend.hopper1.run(?)```

## Errors


| Code | Value |
| --- | ---- |
| 31     | Motor not connected. |
| 32     | Motor jammed. |