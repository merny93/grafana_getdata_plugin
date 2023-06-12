# Datasource API

To plot data in Grafana the user creates *visualizations* within a *dashboard*. Each *visualization* is controlled by a *query* which tells a *datasource* what data needs to be plotted. This document outlines how to use the GetData datasource.

## Setup

To add a datasource click the *toggle menu* in the top left of the Grafan home page and navigate to *Add new connection* under the *Connections* sub-heading. Here you can search *Dirfile* which should pop-up the datasource. Click on it and then click the large blue button in the top right marked *Create a Dirfile (getdata) Datasource data source* (ignore the invalid plugin signature warning).

The following menu allows you to name the datasource and provide a path to the Dirfile (API key is currently not used). *Save & test* will save the settings and attempt to read the `INDEX` field from the Dirfile to confirm it is working.

**Troubleshooting:** If you can not find the datasource make sure that you installed it correctly and that you configured Grafana to load unsigned plugins. See the [backend plugin documentation](https://grafana.com/tutorials/build-a-data-source-backend-plugin/), server logs are also helpful here.

## Query

Once you create a new *dashboard* you can add a *visualization* which will pop up the *query* editor. 

Start by selecting the appropriate *Data source*, this will pull up the getdata query editor with the following fields:

- **Field Name:** This is what data you want to plot on the y-axis. The search field supports regex strings and behaves identically to the KST add data lookup
- **time type:** Casts x-axis value to a `datetime` type. This is required to use *Time series* Visualization. If this is not set you will want to navigate to the top right to switch from a *Time series* visualization to an *XY Chart* (currently in Beta) and configure the appropriate x axis.
- **Time Field Name:** This lets you select what to plot on the X-axis. Unless you select *Index time by INDEX* this field will be used for data selection based off the requested time chunk as set using the Grafana UI (top right of the dashboard)
- **Streaming:** Check box to tell the backend that you want to receive push updates when new data comes in.
- **Index time by INDEX:** Check box to tell the backend to use the reserved `INDEX` field to select what data gets plotted. By selecting this option, *Time Field Name* is ignored in the data selection process however it is still returned as the x-axis. If the *time type* checkbox is selected and *Time Field Name* is set to `INDEX` then the backend will generate a `datetime` object derived from the next options. If this option is selected you will need to fill in the rest of the bottom row.
- **Index time offset type:** Drop down allows you to select how `INDEX` is interpreted
    - **From start:** This tells the backend to assume that the starting index corresponds to whatever time is entered in *Index time offset* and that there are *sample rate* `frames` per second.
    - **From end:** This tells the backend to assume that the last index corresponds to whatever time is entered in *Index time offset* and that there are *sample rate* `frames` per second.
    - **From end now:** This tells the backend to assume that the last index correspond to current `datetime` and that there are *sample rate* `frames per second. This option is likely what you want to use if you are streaming live data as it is robust to glitches and is guaranteed to plot the newest data even if the payload time does not match local time. 

Under *Query options* you will find some other helpful options such as *Max data points* which sets the level of decimation done on the backend. The backend is conservative and will never send more data than is requested but can send less for stupid implementation reasons. This number is also used to compute the *Interval* which represents the maximum frequency at which the backend is allowed to push data when streaming. If you care about fidelity more than performance feel free to increase the *Max data points* significantly. The internal implementation is lossy decimation.

Another implementation detail is how the datasource deals with data which has a `spf>1` (samples per frame) for the y-axis but only 1 `spf` for the x-axis. In this case the backend will interpolate the x-axis to match the y-axis, this matches KST's behavior. 

**Troubleshooting:** If things are not working as expected the backend should push any `getdata` errors to the front end as they arise. If the time-range selector does not appear in the dashboard go to *dashboard settings* and uncheck the *Hide time picker* option under *General*
