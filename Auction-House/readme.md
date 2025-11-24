# Running the program
To run the program do the following:
1. Navigate into ``node`` with ``cd .\node\``
2. Start as many nodes as you want (More can be added later also)
3. Navigate into ``client`` with ``cd .\client\``
4. Start as many clients as you like (More can also be added later)
5. Follow the terminal commands for operation the system. Please remember to "mimic" node failover by using ``.Quit`` in the terminal. **If you forget to do this** there will be anomalies in the system on the next run/event. See _Common Errors_ if you make this mistake

# Common Errors
**Using CTRL+C to close nodes**

To fix this navigate to your systems ``TEMP`` folder and delete ``LiveProcesses.txt``. Then you can run the system like normal again