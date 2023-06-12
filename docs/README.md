# Grafana GetData Plugin

A Grafana plugin which supports the [Dirfile standard](https://getdata.sourceforge.net/dirfile.html) through the reference [GetData implementation](https://github.com/ketiltrout/getdata). This is a [backend datasource plugin](https://grafana.com/docs/grafana/latest/developers/plugins/backend/) which support [streaming](https://grafana.com/docs/grafana/latest/setup-grafana/set-up-grafana-live/) (a new feature rolled out in Grafana 8.0).

| Query sample | Dashboard sample |
| ------------ | ---------------- |
| ![query sample](assets/query_bit.png | width = 100) | ![dashboard sample](assets/dashboard_bit.png | width = 100) |

## Motivations

The Dirfile standard is often used in situation where live data viewing is required such as cryogenics or flight operations for scientific balloon observatories. Currently, to view live data an enterprising scientist has to run a script, known as the `defile` script, which converts an incoming (usually web) stream of binary data into a valid Dirfile and then plot the data using [KST](https://kst-plot.kde.org/). To add further complications, many experiments have designed GUIs to communicate with `defile`. All of these programs were designed to build in Ubuntu and while there exist worked examples of running the software stack in WSL this is clunky and difficult. Finally, the current structure requires everyone to maintain a local copy of the dataset with no provision to sync up old data implying that it is impossible to plot data which was streamed before the local `defile` was run.

The temporary work around which has been implemented by some groups (notably SPIDER and SuperBIT) is to run `defile` and `KST` on a central server which takes screenshots of a hardcoded `KST` session every few minutes and updates a static HTML page (see [SPIDER live-plots](http://labah.princeton.edu/lloro/) or [SuperBIT live-plots](http://labah.princeton.edu/~susan/bit_plots/views/power.html#15)). This solution leaves a lot to be desired: `KST` is a fully fledged data-visualization tool with strong data-processing functionality, but by running it in "screenshot" mode all this functionality is thrown out. Due to the static nature of the website users are not able to dynamically interact with the plots at all, whatever was hardcoded in is all you get.

The Grafana GetData plugin aims to solve the lack of portability created by the current software stack without compromising on the functionality in the same way that current web-plot implementations do.

## Grafana for Science

Grafana is the data-visualization tool of choice for [Simon's Observatory](https://arxiv.org/pdf/2012.10345.pdf). They have confirmed that the functionality offered by Grafana is sufficient to deal with typical workflows created by large scientific collaborations. Grafana supports user created plugins and with such a large scientific community using Grafana new requested are getting attention and functionality is constantly being implemented.

## GetData Plugin

The GetData plugin runs on the backend (computer serving the Grafana resources) and is able to return data from a Dirfile to respond to custom *queries*. Users are able to generate these *queries* from the Grafana UI and combine queries to build *dashboards*. The plugin is able to serve historical data queried either by any time field or by the reserved `INDEX` field. It is also able to enter *streaming* mode in which it will send new data via `web-sockets` as it becomes available. The plugin performs decimation automatically on the backend resulting in excellent performance: querying millions of points does not result in any perceivable delay.

## Install and Requirements

### Development 

This plugin has only been tested on Ubuntu 22.04 LTS, Grafana 10.0.0-preview. It will likely not build in windows... but should work with Grafana 8.0+

The front end components are built with `yarn install; yarn build` thus requiring `npm` to be installed and working on your computer. The backend is written in `golang` and thus requires a working `go` install. Usually Grafana backends are built with `mage` but I could not get that working with `cgo` so it is currently being built with `go build -o dist/gpx_my_plugin_linux_amd64 -v pkg/main.go` note that the `amd64` might need to be changed for your system architecture. The backend is linked against your install of `getdata` which I assumed is not on your `LD_LIBRARY_PATH` but is located in the usual place, if this is not the case you will need to updated the `gcc` flags in `pkg/plugin/getdata.go`. 

For Grafana to find the plugin you must either copy/simlink the parent directory into a place which is on the `grafana-plugins` path or simply just update the path to include your working directory. You will also have to update the `config` file to allow loading of unsigned plugins. More information on these steps can be found in the [backend plugin documentation](https://grafana.com/tutorials/build-a-data-source-backend-plugin/).

Of course you will need a valid `defile` script running to generate data...

### Production

You should probably talk to a systems administrator here, nothing is fundamentally different but you need to open port 3000 to external traffic.