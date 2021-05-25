# a10-http-error-monitor
Watch for HTTP access errors from an SLB configured on an A10 Thunder and send out an Alert via MQTT

This is more of a demo than a serious tool, but the concept here is that I can monitor the Syslog records coming out of
an A10 Thunder device for specific records that were created by the Aflex script when it catches a 4xx or 5xx HTTP Error
Code. When it sees one of these records come across, it will capture the message and send it out as an 'Alert' message via 
the MQTT protocol.

This program makes use of the A10 Thunder logging mechanism to output RFC 3164 Syslog messages, which this program reads. An aFleX script is assigned to the SLB that you need to monitor, and it looks for these HTTP Status Codes of 4xx to 5xx and logs a message that this program picks up.

While I used MQTT for my "Alerts", the program could be changed to use anything from Email to Slack.  I use MQTT because my Home Lab Alert system is built on it ;)
