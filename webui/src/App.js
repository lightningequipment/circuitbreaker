import React, {useState, useEffect, useRef } from 'react';
import logo from './logo.svg';
import './App.css';
import { DataTable } from 'primereact/datatable';
import { Panel } from 'primereact/panel';
import { Column } from 'primereact/column';
import { ColumnGroup } from 'primereact/columngroup';
import { InputNumber } from 'primereact/inputnumber';
import { Button } from 'primereact/button';
import { Row } from 'primereact/row';
import { Tooltip } from 'primereact/tooltip';

import "primereact/resources/themes/lara-light-indigo/theme.css";  //theme
import "primereact/resources/primereact.min.css";                  //core css
import "primeicons/primeicons.css";                                //icons
import moment, { HTML5_FMT } from 'moment';

function App() {
  const [data,setData] = useState([]);
  const [dynamicColumns,setDynamicColumns] = useState();
  const tooltipRef = useRef(null);

  const fetchData = async () => {
    try {
      const response = await fetch("/api/limits");
      const json = await response.json();

      console.log(json)

      const tableData = json.limits.map((l) => {
        const tableRow = {
          alias: l.alias,
          node: l.node,

          counter1h_success: l.counter1h.success,
          counter1h_fail: l.counter1h.fail,
          counter1h_reject: l.counter1h.reject,
          counter24h_success: l.counter24h.success,
          counter24h_fail: l.counter24h.fail,
          counter24h_reject: l.counter24h.reject,

          maxPending: l.limit.maxPending,
          maxHourlyRate: l.limit.maxHourlyRate,
        };

        if (l.alias == "") {
          tableRow.alias = l.node.slice(0, 8) + '...' + l.node.slice(58, 66)
        }

        return tableRow
      })

      setData(tableData)
    } catch (error) {
      console.log("error", error);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  useEffect(() => {
    tooltipRef.current && tooltipRef.current.updateTargetEvents();
  }, [data]);

  const bodyTemplate = (rowData) => {
    return <div className="custom-tooltip" data-pr-tooltip={rowData.node}>{rowData.alias}</div>
  }

  let headerGroup = <ColumnGroup>
                        <Row>
                            <Column header="Peer" field="alias" rowSpan={2} sortable/>

                            <Column header="Last 1 hr" colSpan={3}/>
                            <Column header="Last 24 hr" colSpan={3}/>

                            <Column header="Limits" colSpan={3}/>
                        </Row>
                        <Row>
                            <Column header="OK" field="counter1h_success" sortable />
                            <Column header="Fail" field="counter1h_fail" sortable />
                            <Column header="Rej" field="counter1h_reject" sortable />
                            <Column header="OK" field="counter24h_success" sortable />
                            <Column header="Fail" field="counter24h_fail" sortable />
                            <Column header="Rej" field="counter24h_reject" sortable />

                            <Column header="Max Hourly Rate" field="limit.maxHourlyRate" sortable />
                            <Column header="Max Pending" field="limit.maxPending" sortable />
                            <Column />
                        </Row>
                    </ColumnGroup>;

  const textEditor = (options) => {
    return <InputNumber value={options.value} onValueChange={(e) => options.editorCallback(e.value)} size="8"/>
  }

  const onRowEditComplete = (e) => {
    let { newData, index } = e;

    console.log(index)
    console.log(newData)


    const requestOptions = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ maxHourlyRate: newData.maxHourlyRate, maxPending: newData.maxPending })
    };

    fetch('/api/updatelimit/'+newData.node, requestOptions)
      .then(response => response.json())
      .then(data => {
        fetchData();
        console.log(data)
      });

  }

  return (
    <Panel header="Limits">
    <Tooltip ref={tooltipRef} target=".custom-tooltip"></Tooltip>
    <DataTable value={data} responsiveLayout="scroll" sortField="node" sortOrder={1} headerColumnGroup={headerGroup}  editMode="row" onRowEditComplete={onRowEditComplete} >
      <Column field="node" body={bodyTemplate}></Column>
      <Column field="counter1h_success"></Column>
      <Column field="counter1h_fail"></Column>
      <Column field="counter1h_reject"></Column>
      <Column field="counter24h_success"></Column>
      <Column field="counter24h_fail"></Column>
      <Column field="counter24h_reject"></Column>
      <Column field="maxHourlyRate" editor={(options) => textEditor(options)}></Column>
      <Column field="maxPending" editor={(options) => textEditor(options)}></Column>

      <Column rowEditor></Column>
    </DataTable>
    </Panel>


  );
}

export default App;
