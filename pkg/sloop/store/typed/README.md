# Tables in Sloop 


<img src="https://github.com/salesforce/sloop/raw/master/other/sloop_logo_color_small_notext.png">

----

There are four tables in Sloop to store data:

1. Watch table
1. Resources summary table
1. Event count table
1. Watch activity table

----

Details:

1. Watch table:
It has the raw kube watch data. It is the source of truth for the whole data. 

1. Resource Summary: It stores the resources information including name, creation date, deployment details and last update time.

1. Event table: It stores the event details that took place for a resource. The information includes event type, message and time stamp.

1. Watch Activity table: It stores any watch activity received. It has the information that was there a change from the last known state or not.


## Data Distribution

The data distribution in terms of size among the tables is shown below. As expected, watch table occupies the most space as it contains the raw data. Rest of the tables are derived from it.
![DataDistribution](../../../../other/data_distribution.png?raw=true "Data Distribution among Sloop tables")
