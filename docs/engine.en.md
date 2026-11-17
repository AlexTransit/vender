# Engine Commands.
English | [Русский](engine.ru.md)


| Command | Arguments | Description |
| :--- | :--- | :--- |
| `error(?)` | Optional text | Returns and forwards a scenario or alias error. The script execution will halt if there is no dedicated error handling script. |
| `error.log(?)` | Optional text | Forwards an error text to the server. Error type is **error**. The script continues execution. |
| `info.log(?)` | Optional text | Forwards an informational text to the server. Error type is **info**. The script continues execution. |
| `wait.ok(?)` | int16 (seconds) | Waits for the Ok button to be pressed for the specified duration. If no press occurs, it returns error **99**. |
| `vmc.stop!` | - | Terminates the program execution immediately. |
| `vmc.upgrade!` | ```upgrade_script = "sudo -u vmc git -C /home..."``` | Executes the script defined in the configuration. Upon successful completion of the script, **`vmc.stop!`** is called right before the `on_front_begin` scenario starts. |
| `check.menu` | - | Checks the menu structure for errors (e.g., incorrect syntax or selling price below cost). |
| `write.config` | - | Creates the default configuration file, ```defaultConfig.hcl```. |
| `menu.cost` | - | Displays the current menu structure: ```code;price;cost;name;scenario```. |
| `line1(?)` | text | Outputs text to the first line of the text display. Use `_` character instead of a space. |
| `line2(?)` | text | Output to the second line of the text display. |
| `set.broken!` | - | Sets the machine to a "broken" state. To reset this state, service mode must be entered. Power cycling does not reset the state. |
| `picture(?)` | raw file. Must specify the full path | Copies the content of the specified raw file into the screen buffer. |
| `emulate.key(?)` | number | Emulates a keyboard key press. 0-9 for digits. 10 - C. 11 - Ok. 12 - Cream Minus. 13 - Cream Plus. 14 - Sugar Minus. 16 - Sugar Plus/Dot. |
| `mdb.bus_reset` | - | Performs a reset on the MDB bus. |
| `money.commit` | - | Financially commits the transaction, assuming the drink is prepared. This prevents the refund of the paid amount due to subsequent script errors. |
| `stock.dump` | - | Dumps current stock data. |