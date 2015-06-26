Instructions
============

Usage
-----

Just create a file **azure_sql_metrics.json** and follow this template (Shown values are the default ones):

.. code::

    {
      "server": "localhost",
      "port": "1433",
      "user": "sa",
      "password": "",
      "schema": "master",
      "ignoreIps": ["0.0.0.0"] 
    }

Where:

*server*
    Hostname where the Sql Server is running
*port*
    Port wher SQL Server is running
*user*
    User to be authenticated.
*password*
    Password to be used in the authentication.
*schema*
    dot separated values to indicate the path to Graphite/Grafana
*ignoreIps*
    List of IPs to be ignored
    
Then you can run the executable with:

.. code::

    go run 
    
    
Compilation
-----------

1) Install Go SDK: ``cinst golang``
2) Ensure you have the environment variable ``%GOPATH%``
3) install go-mssqldb package: ``go get github.com/denisenkom/go-mssqldb``
4) Run it: ``go run azure_sql_metrics.go -h``
5) Compile it: ``go build azure_sql_metrics.go``
6) Run it compiled: ``azure_sql_metrics.exe -h``