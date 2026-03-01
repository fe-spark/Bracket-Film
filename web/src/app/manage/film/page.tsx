"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Table,
  Tag,
  Button,
  Space,
  Select,
  Input,
  DatePicker,
  Popconfirm,
  Tooltip,
  Pagination,
} from "antd";
import { useRouter } from "next/navigation";
import {
  SearchOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  AimOutlined,
  FireOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ApiGet } from "@/lib/api";
import dayjs from "dayjs";
import styles from "./page.module.less";
import { useAppMessage } from "@/lib/useAppMessage";

const { RangePicker } = DatePicker;

interface FilmItem {
  mid: number;
  ID: number;
  name: string;
  cName: string;
  year: string | number;
  score: string | number;
  hits: number;
  remarks: string;
  updateStamp: number;
}

export default function FilmListPage() {
  const router = useRouter();
  const [list, setList] = useState<FilmItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState({ current: 1, pageSize: 10, total: 0 });

  // Filters
  const [params, setParams] = useState<any>({
    name: "",
    pid: 0,
    cid: 0,
    plot: "",
    area: "",
    language: "",
    year: "",
    remarks: "",
    beginTime: "",
    endTime: "",
  });

  const [options, setOptions] = useState<any>({
    class: [],
    Plot: [],
    Area: [],
    Language: [],
    year: [],
    remarks: [],
    tags: {},
  });

  const [classId, setClassId] = useState<number>(0);
  const [dateRange, setDateRange] = useState<any>(null);

  const { message } = useAppMessage();

  const getFilmPage = useCallback(
    async (p?: any) => {
      setLoading(true);
      const pg = p || page;
      try {
        const resp = await ApiGet("/manage/film/search/list", {
          ...params,
          current: pg.current,
          pageSize: pg.pageSize,
        });
        if (resp.code === 0) {
          const formattedList = (resp.data.list || []).map((item: any) => ({
            ...item,
            year: item.year <= 0 ? "未知" : item.year,
            score: item.score === 0 ? "暂无" : item.score,
          }));
          setList(formattedList);
          setPage(resp.data.params.paging);

          // Update options if provided
          if (resp.data.options) {
            setOptions((prev: any) => ({
              ...prev,
              ...resp.data.options,
            }));
          }
        }
      } finally {
        setLoading(false);
      }
    },
    [params, page],
  );

  useEffect(() => {
    getFilmPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleClassChange = (value: number) => {
    setClassId(value);
    const selectedClass = options.class.find((c: any) => c.id === value);
    if (!selectedClass) return;

    const newParams = { ...params };
    if (selectedClass.pid <= 0) {
      newParams.pid = selectedClass.id;
      newParams.cid = 0;
    } else {
      newParams.pid = selectedClass.pid;
      newParams.cid = selectedClass.id;
    }

    // Handle tags (Plot, Area, Language)
    const t =
      selectedClass.pid === 0
        ? options.tags[selectedClass.id]
        : options.tags[selectedClass.pid];
    if (t) {
      setOptions((prev: any) => ({
        ...prev,
        Plot: t.Plot || [],
        Area: t.Area || [],
        Language: t.Language || [],
      }));
    } else {
      setOptions((prev: any) => ({
        ...prev,
        Plot: [],
        Area: [],
        Language: [],
      }));
    }

    newParams.plot = "";
    newParams.area = "";
    newParams.language = "";
    setParams(newParams);
  };

  const onSearch = () => {
    const p = { ...params };
    if (dateRange && dateRange[0] && dateRange[1]) {
      p.beginTime = dateRange[0].format("YYYY-MM-DD HH:mm:ss");
      p.endTime = dateRange[1].format("YYYY-MM-DD HH:mm:ss");
    } else {
      p.beginTime = "";
      p.endTime = "";
    }
    setParams(p);
    setPage({ ...page, current: 1 });
    // Trigger fetch in effect or here manually
    getFilmPage({ ...page, current: 1 });
  };

  const handleUpdateSingle = async (mid: number) => {
    const resp = await ApiGet("/manage/spider/update/single", { ids: mid });
    if (resp.code === 0) message.success(resp.msg);
    else message.error(resp.msg);
  };

  const handleDelFilm = async (id: number) => {
    const resp = await ApiGet("/manage/film/search/del", { id });
    if (resp.code === 0) {
      message.success(resp.msg);
      getFilmPage();
    } else message.error(resp.msg);
  };

  const columns: ColumnsType<FilmItem> = [
    {
      title: "序号",
      key: "index",
      width: 60,
      render: (_, __, index) => (
        <span style={{ color: "#8b40ff" }}>
          {(page.current - 1) * page.pageSize + index + 1}
        </span>
      ),
    },
    {
      title: "影片ID",
      dataIndex: "mid",
      align: "center",
      render: (v) => <Tag color="success">{v}</Tag>,
    },
    {
      title: "影片名称",
      dataIndex: "name",
      ellipsis: true,
      className: styles.filmName,
    },
    {
      title: "所属分类",
      dataIndex: "cName",
      align: "center",
      render: (v) => <Tag color="orange">{v}</Tag>,
    },
    {
      title: "年份",
      dataIndex: "year",
      align: "center",
      render: (v) => <Tag color="orange">{v}</Tag>,
    },
    {
      title: "评分",
      dataIndex: "score",
      align: "center",
      sorter: (a, b) => Number(a.score) - Number(b.score),
      render: (v) => <Tag color="processing">{v}</Tag>,
    },
    {
      title: "热度",
      dataIndex: "hits",
      align: "center",
      sorter: (a, b) => a.hits - b.hits,
      render: (v) => (
        <Tag color="error">
          <FireOutlined /> {v}
        </Tag>
      ),
    },
    {
      title: "更新状态",
      dataIndex: "remarks",
      align: "center",
      render: (v) => (
        <Tag color={v.includes("更新") ? "warning" : "success"}>{v}</Tag>
      ),
    },
    {
      title: "更新时间",
      dataIndex: "updateStamp",
      align: "center",
      sorter: (a, b) => a.updateStamp - b.updateStamp,
      render: (v) => (
        <Tag color="success">
          {dayjs(v * 1000).format("YYYY-MM-DD HH:mm:ss")}
        </Tag>
      ),
    },
    {
      title: "操作",
      key: "action",
      align: "center",
      width: 150,
      fixed: "right",
      render: (_, record) => (
        <Space>
          <Tooltip title="详情预览">
            <Button
              type="primary"
              icon={<AimOutlined />}
              shape="circle"
              size="small"
              ghost
              onClick={() =>
                window.open(`/filmDetail?link=${record.mid}`, "_blank")
              }
            />
          </Tooltip>
          <Tooltip title="同步更新">
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              shape="circle"
              size="small"
              style={{ background: "#52c41a", borderColor: "#52c41a" }}
              onClick={() => handleUpdateSingle(record.mid)}
            />
          </Tooltip>
          <Tooltip title="修改影视">
            <Button
              type="primary"
              icon={<EditOutlined />}
              shape="circle"
              size="small"
              onClick={() => router.push(`/manage/film/add?id=${record.mid}`)}
            />
          </Tooltip>
          <Popconfirm
            title="确认删除此影片？"
            onConfirm={() => handleDelFilm(record.ID)}
          >
            <Button
              danger
              icon={<DeleteOutlined />}
              shape="circle"
              size="small"
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className={styles.container}>
      <div className={styles.filterBar}>
        <Input
          placeholder="片名搜索"
          prefix={<SearchOutlined />}
          value={params.name}
          onChange={(e) => setParams({ ...params, name: e.target.value })}
          style={{ width: 180 }}
        />
        <Select
          placeholder="影片分类"
          className={styles.filterItem}
          value={classId || undefined}
          onChange={handleClassChange}
          options={options.class.map((c: any) => ({
            label: c.name,
            value: c.id,
          }))}
          allowClear
        />
        <Select
          placeholder="剧情筛选"
          className={styles.filterItem}
          value={params.plot || undefined}
          onChange={(v) => setParams({ ...params, plot: v })}
          options={(options.Plot || []).map((i: any) => ({
            label: i.Name,
            value: i.Value,
          }))}
          allowClear
        />
        <Select
          placeholder="地区筛选"
          className={styles.filterItem}
          value={params.area || undefined}
          onChange={(v) => setParams({ ...params, area: v })}
          options={(options.Area || []).map((i: any) => ({
            label: i.Name,
            value: i.Value,
          }))}
          allowClear
        />
        <Select
          placeholder="语言筛选"
          className={styles.filterItem}
          value={params.language || undefined}
          onChange={(v) => setParams({ ...params, language: v })}
          options={(options.Language || []).map((i: any) => ({
            label: i.Name,
            value: i.Value,
          }))}
          allowClear
        />
        <Select
          placeholder="上映年份"
          className={styles.filterItem}
          value={params.year || undefined}
          onChange={(v) => setParams({ ...params, year: v })}
          options={(options.year || []).map((i: any) => ({
            label: i.Name,
            value: i.Value,
          }))}
          allowClear
        />
        <Select
          placeholder="更新状态"
          className={styles.filterItem}
          value={params.remarks || undefined}
          onChange={(v) => setParams({ ...params, remarks: v })}
          options={(options.remarks || []).map((i: any) => ({
            label: i.Name,
            value: i.Value,
          }))}
          allowClear
        />
        <RangePicker
          showTime
          value={dateRange}
          onChange={(v) => setDateRange(v)}
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={onSearch}>
          查询
        </Button>
      </div>

      <div className={styles.tableContainer}>
        <Table
          title={() => (
            <div style={{ display: "flex", justifyContent: "flex-end" }}>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => router.push("/manage/film/add")}
                className={styles.addFilmBtn}
              >
                新增影片
              </Button>
            </div>
          )}
          columns={columns}
          dataSource={list}
          rowKey="mid"
          loading={loading}
          bordered
          size="middle"
          pagination={false}
          scroll={{ x: "max-content" }}
        />
        <div className={styles.paginationContainer}>
          <Pagination
            current={page.current}
            pageSize={page.pageSize}
            total={page.total}
            showSizeChanger
            showTotal={(total) => `共 ${total} 条`}
            onChange={(current, pageSize) => {
              const newPage = { ...page, current, pageSize };
              setPage(newPage);
              getFilmPage(newPage);
            }}
          />
        </div>
      </div>
    </div>
  );
}
