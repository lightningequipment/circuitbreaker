import React, { useState, useEffect, useRef } from 'react';
import logo from './logo.svg';
import './App.css';
import { DataTable } from 'primereact/datatable';
import { Card } from 'primereact/card';
import { Column } from 'primereact/column';
import { ColumnGroup } from 'primereact/columngroup';
import { InputNumber } from 'primereact/inputnumber';
import { Row } from 'primereact/row';
import { Tooltip } from 'primereact/tooltip';
import { Dropdown } from 'primereact/dropdown';
import { Button } from 'primereact/button';
import { Panel } from 'primereact/panel';
import { Divider } from 'primereact/divider';
import { Messages } from 'primereact/messages';
import { Message } from 'primereact/message';

import "primereact/resources/themes/lara-light-indigo/theme.css";  //theme
import "primereact/resources/primereact.min.css";                  //core css
import "primeicons/primeicons.css";                                //icons
import moment, { HTML5_FMT } from 'moment';

function App() {
  const [data, setData] = useState([]);
  const [defaultLimits, setDefaultLimits] = useState([]);
  const [dynamicColumns, setDynamicColumns] = useState();
  const tooltipRef = useRef(null);
  const warningMsg = useRef(null);
  const defaultTableEditing = useRef(false);
  const nodeTableEditing = useRef(false);

  const fetchData = async () => {
    try {
      const response = await fetch("/api/limits");
      const json = await response.json();

      console.log(json)

      let warning = false

      const tableData = json.limits.map((l) => {
        const tableRow = {
          alias: l.alias,
          node: l.node,
          pendingHtlcCount: l.pendingHtlcCount,
          counter1h_success: l.counter1h.success,
          counter1h_fail: l.counter1h.fail,
          counter1h_reject: l.counter1h.reject,
          counter24h_success: l.counter24h.success,
          counter24h_fail: l.counter24h.fail,
          counter24h_reject: l.counter24h.reject,

          queueLen: l.queueLen,

          mode: 'MODE_DEFAULT',
          maxPending: 0,
          maxHourlyRate: 0,
        };

        if (l.limit !== null) {
          tableRow.mode = l.limit.mode
          tableRow.maxPending = l.limit.maxPending
          tableRow.maxHourlyRate = l.limit.maxHourlyRate

          if (l.limit.mode == 'MODE_QUEUE' || l.limit.mode == 'MODE_QUEUE_PEER_INITIATED') {
            warning = true
          }
        }

        if (l.alias == "") {
          tableRow.alias = l.node.slice(0, 8) + '...' + l.node.slice(58, 66)
        }


        return tableRow
      })

      if (json.defaultLimit.mode == 'MODE_QUEUE' || json.defaultLimit.mode == 'MODE_QUEUE_PEER_INITIATED') {
        warning = true
      }

      let msgs = []

      if (warning) {
        msgs.push({ sticky: true, severity: 'warn', summary: 'Warning: if something goes wrong in queue mode, this may lead to channel force-closes in lnd 0.15 and below', closable: false })
      }

      warningMsg.current.replace(msgs);

      setData(tableData)
      setDefaultLimits([json.defaultLimit])
    } catch (error) {
      console.log("error", error);
    }
  };

  useEffect(() => {
    fetchData();

    const interval = setInterval(() => {
      if (defaultTableEditing.current || nodeTableEditing.current) {
        console.log('skip update because editing')

        return
      }

      fetchData();
    }, 60000)

    return () => clearInterval(interval)
  }, []);

  useEffect(() => {
    tooltipRef.current && tooltipRef.current.updateTargetEvents();
  }, [data]);

  const bodyTemplate = (rowData) => {
    return <div className="custom-tooltip" data-pr-tooltip={rowData.node}>{rowData.alias}</div>
  }


  const textEditor = (options) => {
    return <InputNumber value={options.value} onValueChange={(e) => options.editorCallback(e.value)} size="8" useGrouping={false} />
  }

  const onDefaultLimitEditComplete = (e) => {
    let { newData, index } = e;

    const requestOptions = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        limit: {
          maxHourlyRate: newData.maxHourlyRate,
          maxPending: newData.maxPending,
          mode: newData.mode
        }
      })
    };

    fetch('/api/updatedefaultlimit', requestOptions)
      .then(response => response.json())
      .then(data => {
        console.log(data)

        fetchData();
      });
  }

  const onRowEditComplete = (e) => {
    let { newData, index } = e;

    if (newData.mode == 'MODE_DEFAULT') {
      const requestOptions = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      };

      fetch('/api/clearlimit/' + newData.node, requestOptions)
        .then(response => response.json())
        .then(data => {
          console.log(data)

          fetchData();
        });
    } else {
      const requestOptions = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          limit: {
            maxHourlyRate: newData.maxHourlyRate,
            maxPending: newData.maxPending,
            mode: newData.mode
          }
        })
      };

      fetch('/api/updatelimit/' + newData.node, requestOptions)
        .then(response => response.json())
        .then(data => {
          console.log(data)

          fetchData();
        });
    }
  }

  const modes = [
    { label: 'Fail', value: 'MODE_FAIL' },
    { label: 'Queue', value: 'MODE_QUEUE' },
    { label: 'Queue peer initiated', value: 'MODE_QUEUE_PEER_INITIATED' }
  ];

  const modesWithUseDefault = [...modes, { label: 'Use default', value: 'MODE_DEFAULT' }]

  const modeEditor = (options, addUseDefault) => {
    let thisModes = modes
    if (addUseDefault) {
      thisModes = modesWithUseDefault
    }
    return (
      <Dropdown value={options.value} options={thisModes} optionLabel="label" optionValue="value"
        onChange={(e) => options.editorCallback(e.value)} placeholder="Select a Mode" />
    );
  }

  const modeBodyTemplate = (rowData) => {
    let mode = 'NA'
    switch (rowData.mode) {
      case 'MODE_FAIL':
        mode = 'Fail'
        break;

      case 'MODE_QUEUE':
        mode = 'Queue'
        break;

      case 'MODE_QUEUE_PEER_INITIATED':
        mode = 'Queue peer initiated'
        break;

      case 'MODE_DEFAULT':
        mode = 'Use default'
        break;
    }

    return mode
  }

  const numberBodyTemplate = (mode, number) => {
    if (mode == 'MODE_DEFAULT') {
      return "Use default"
    }

    if (number == 0) return "None"

    return number
  }

  const dashNumberBodyTemplate = (number) => {
    if (number == 0) return "-"

    return number
  }



  let headerGroup = (
    <ColumnGroup>
      <Row>
        <Column header="Peer" field="alias" rowSpan={2} sortable />

        <Column header="Last 1 hr counters" colSpan={3} />
        <Column header="Last 24 hr counters" colSpan={3} />

        <Column header="Current fwds" field="queueLen" colSpan={2} />

        <Column header="Limits" colSpan={5} />
      </Row>
      <Row>
        <Column header="OK" headerTooltip="Number of settled htlcs in the past 1 hr" field="counter1h_success" sortable />
        <Column header="Fail" headerTooltip="Number of failed htlcs in the past 1 hr" field="counter1h_fail" sortable />
        <Column header="Rej" headerTooltip="Number of htlcs rejected by circuit breaker in the past 1 hr" field="counter1h_reject" sortable />
        <Column header="OK" headerTooltip="Number of settled htlcs in the past 24 hr" field="counter24h_success" sortable />
        <Column header="Fail" headerTooltip="Number of failed htlcs in the past 24 hr" field="counter24h_fail" sortable />
        <Column header="Rej" headerTooltip="Number of htlcs rejected by circuit breaker in the past 24 hr" field="counter24h_reject" sortable />

        <Column header="Queue len" field="queueLen" sortable />
        <Column header="Pending" field="pendingHtlcCount" sortable />

        <Column header="Max Hourly Rate" headerTooltip="Maximum number of htlcs forwarded per hour" field="maxHourlyRate" sortable />
        <Column header="Max Pending" headerTooltip="Maximum number of pending htlcs" field="maxPending" sortable />
        <Column header="Mode" field="mode" sortable />
        <Column />
      </Row>
    </ColumnGroup>);

  return (
    <Card title="Circuit Breaker">
      <Tooltip ref={tooltipRef} target=".custom-tooltip"></Tooltip>

      <h3>Default limit</h3>
      <DataTable value={defaultLimits} editMode="row" onRowEditComplete={onDefaultLimitEditComplete} size="small" onRowEditInit={() => defaultTableEditing.current = true} onRowEditCancel={() => defaultTableEditing.current = false} onRowEditSave={() => defaultTableEditing.current = false}>
        <Column header="Max Hourly Rate" field="maxHourlyRate" body={(rowData => numberBodyTemplate(rowData.mode, rowData.maxHourlyRate))} editor={(options) => textEditor(options)}></Column>
        <Column header="Max Pending" field="maxPending" body={(rowData => numberBodyTemplate(rowData.mode, rowData.maxPending))} editor={(options) => textEditor(options)}></Column>
        <Column header="Mode" field="mode" body={modeBodyTemplate} editor={(options) => modeEditor(options, false)}></Column>
        <Column rowEditor></Column>
      </DataTable>

      <h3 style={{ paddingTop: '2rem' }}>Per node limits</h3>
      <DataTable value={data} responsiveLayout="scroll" sortField="node" sortOrder={1} headerColumnGroup={headerGroup} editMode="row" onRowEditComplete={onRowEditComplete} size="small" onRowEditInit={() => nodeTableEditing.current = true} onRowEditCancel={() => nodeTableEditing.current = false} onRowEditSave={() => nodeTableEditing.current = false}>
        <Column field="node" body={bodyTemplate}></Column>
        <Column field="counter1h_success" body={(rowData) => dashNumberBodyTemplate(rowData.counter1h_success)}></Column>
        <Column field="counter1h_fail" body={(rowData) => dashNumberBodyTemplate(rowData.counter1h_fail)}></Column>
        <Column field="counter1h_reject" body={(rowData) => dashNumberBodyTemplate(rowData.counter1h_reject)}></Column>
        <Column field="counter24h_success" body={(rowData) => dashNumberBodyTemplate(rowData.counter24h_success)}></Column>
        <Column field="counter24h_fail" body={(rowData) => dashNumberBodyTemplate(rowData.counter24h_fail)}></Column>
        <Column field="counter24h_reject" body={(rowData) => dashNumberBodyTemplate(rowData.counter24h_reject)}></Column>

        <Column field="queueLen" body={(rowData) => dashNumberBodyTemplate(rowData.queueLen)}></Column>
        <Column field="pendingHtlcCount" body={(rowData) => dashNumberBodyTemplate(rowData.pendingHtlcCount)}></Column>

        <Column field="maxHourlyRate" body={(rowData => numberBodyTemplate(rowData.mode, rowData.maxHourlyRate))} editor={(options) => textEditor(options)}></Column>
        <Column field="maxPending" body={(rowData => numberBodyTemplate(rowData.mode, rowData.maxPending))} editor={(options) => textEditor(options)}></Column>
        <Column field="mode" body={modeBodyTemplate} editor={(options) => modeEditor(options, true)}></Column>
        <Column rowEditor></Column>
      </DataTable>

      <Messages ref={warningMsg} />
    </Card>


  );
}

export default App;
