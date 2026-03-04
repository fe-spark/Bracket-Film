"use client";

import React, { useState, useEffect, useCallback, useRef, useMemo } from "react";
import {
  Table,
  Tag,
  Switch,
  Select,
  Button,
  Space,
  Modal,
  Input,
  Form,
  Popconfirm,
  Checkbox,
  InputNumber,
  Radio,
  Tooltip,
  Alert,
} from "antd";
import {
  PlusOutlined,
  SendOutlined,
  ReloadOutlined,
  DeleteOutlined,
  EditOutlined,
  PoweroffOutlined,
  PauseOutlined,
  LoadingOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ApiGet, ApiPost } from "@/lib/api";
import styles from "./page.module.less";
import { useAppMessage } from "@/lib/useAppMessage";

interface FilmSource {
  id: string;
  name: string;
  uri: string;
  resultModel: number;
  collectType: number;
  collectTypeText: string;
  syncPictures: boolean;
  state: boolean;
  grade: number;
  interval: number;
  cd?: number;
}

const collectDuration = [
  { label: "采集今日", time: 24 },
  { label: "采集三天", time: 72 },
  { label: "采集一周", time: 168 },
  { label: "采集半月", time: 360 },
  { label: "采集一月", time: 720 },
  { label: "采集三月", time: 2160 },
  { label: "采集半年", time: 4320 },
  { label: "全量采集", time: -1 },
];

export default function CollectManagePage() {
  const [siteList, setSiteList] = useState<FilmSource[]>([]);
  const [activeCollectIds, setActiveCollectIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const timerRef = useRef<NodeJS.Timeout | null>(null);
  const { message } = useAppMessage();

  // 主站是否已有采集数据（从服务端获取）
  const [hasMasterData, setHasMasterData] = useState(false);

  // 批量采集弹窗
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchIds, setBatchIds] = useState<string[]>([]);
  const [batchTime, setBatchTime] = useState(24);
  const [batchOptions, setBatchOptions] = useState<any[]>([]);

  // 添加/编辑弹窗
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [form] = Form.useForm();
  const [editingId, setEditingId] = useState<string | null>(null);

  // 清空/重采弹窗
  const [clearOpen, setClearOpen] = useState(false);
  const [reCollectOpen, setReCollectOpen] = useState(false);
  const [password, setPassword] = useState("");

  // batchOptions 追加 grade 信息（与 siteList 合并），并过滤掉主站
  const enrichedBatchOptions = useMemo(
    () =>
      batchOptions
        .map((o) => ({
          ...o,
          grade: siteList.find((s) => s.id === o.id)?.grade ?? 1,
        }))
        .filter((o) => o.grade !== 0),
    [batchOptions, siteList]
  );

  // 主站是否正在采集中
  const masterIsCollecting = useMemo(
    () => siteList.filter((s) => s.grade === 0).some((s) => activeCollectIds.includes(s.id)),
    [siteList, activeCollectIds]
  );

  // 从站可采集条件：主站已有数据 且 主站当前未在采集
  const slaveCanCollect = hasMasterData && !masterIsCollecting;

  const getCollectList = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await ApiGet("/manage/collect/list");
      if (resp.code === 0) {
        const list = resp.data?.map((item: any) => {
          let typeText = "视频";
          switch (item.collectType) {
            case 1:
              typeText = "文章";
              break;
            case 2:
              typeText = "演员";
              break;
            case 3:
              typeText = "角色";
              break;
            case 4:
              typeText = "网站";
              break;
          }
          return {
            ...item,
            collectTypeText: typeText,
            cd: item.cd || 24,
          };
        });
        setSiteList(list);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const getCollectingState = useCallback(async () => {
    const resp = await ApiGet("/manage/collect/collecting/state", undefined);
    if (resp.code === 0 && resp.data) {
      setActiveCollectIds(resp.data);
    }
  }, []);

  const getMasterDataStatus = useCallback(async () => {
    const resp = await ApiGet("/manage/spider/master/status");
    if (resp.code === 0) setHasMasterData(resp.data === true);
  }, []);

  useEffect(() => {
    getCollectList();
    getCollectingState();
    getMasterDataStatus();
    timerRef.current = setInterval(() => {
      getCollectingState();
      getMasterDataStatus();
    }, 4000);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [getCollectList, getCollectingState, getMasterDataStatus]);

  const changeSourceState = async (record: FilmSource) => {
    const resp = await ApiPost("/manage/collect/change", {
      id: record.id,
      state: record.state,
      syncPictures: record.syncPictures,
    });
    if (resp.code !== 0) message.error(resp.msg);
  };

  const startTask = async (record: FilmSource) => {
    const resp = await ApiPost("/manage/spider/start", {
      id: record.id,
      time: record.cd || 24,
      batch: false,
    });
    if (resp.code === 0) {
      message.success(resp.msg);
      getCollectingState();
    } else {
      message.error(resp.msg);
    }
  };

  const stopTask = async (id: string) => {
    const resp = await ApiGet("/manage/collect/stop", { id });
    if (resp.code === 0) {
      message.success("已请求停止任务");
      getCollectingState();
    }
  };

  const delSource = async (id: string) => {
    const resp = await ApiGet("/manage/collect/del", { id });
    if (resp.code === 0) {
      message.success(resp.msg);
      getCollectList();
    } else {
      message.error(resp.msg);
    }
  };

  const openAddDialog = () => {
    form.resetFields();
    form.setFieldsValue({
      resultModel: 0,
      grade: 1,
      collectType: 0,
      syncPictures: false,
      state: false,
      interval: 0,
    });
    setAddOpen(true);
  };

  const openEditDialog = async (id: string) => {
    setEditingId(id);
    const resp = await ApiGet("/manage/collect/find", { id });
    if (resp.code === 0) {
      form.setFieldsValue(resp.data);
      setEditOpen(true);
    } else {
      message.error(resp.msg);
    }
  };

  const onAddFinish = async (values: any) => {
    const resp = await ApiPost("/manage/collect/add", values);
    if (resp.code === 0) {
      message.success(resp.msg);
      setAddOpen(false);
      getCollectList();
    } else {
      message.error(resp.msg);
    }
  };

  const onEditFinish = async (values: any) => {
    const resp = await ApiPost("/manage/collect/update", {
      ...values,
      id: editingId,
    });
    if (resp.code === 0) {
      message.success(resp.msg);
      setEditOpen(false);
      getCollectList();
    } else {
      message.error(resp.msg);
    }
  };

  const testApi = async () => {
    try {
      const values = await form.validateFields();
      const resp = await ApiPost("/manage/collect/test", values);
      if (resp.code === 0) message.success(resp.msg);
      else message.error(resp.msg);
    } catch { }
  };

  const openBatchCollect = async () => {
    setBatchOpen(true);
    const resp = await ApiGet("/manage/collect/options");
    if (resp.code === 0) setBatchOptions(resp.data || []);
  };

  const startBatchCollect = async () => {
    if (batchIds.length === 0) {
      message.warning("请至少选择一个站点");
      return;
    }
    const resp = await ApiPost("/manage/spider/start", {
      ids: batchIds,
      time: batchTime,
      batch: true,
    });
    if (resp.code === 0) {
      message.success(resp.msg);
      setBatchOpen(false);
      getCollectingState();
    } else {
      message.error(resp.msg);
    }
  };

  const clearFilms = async () => {
    if (!password) {
      message.error("请输入密钥");
      return;
    }
    const resp = await ApiGet("/manage/spider/clear", { password });
    if (resp.code === 0) message.success(resp.msg);
    else message.error(resp.msg);
    setClearOpen(false);
    setPassword("");
  };

  const reCollect = async () => {
    if (!password) {
      message.error("请输入密钥");
      return;
    }
    const resp = await ApiGet("/manage/spider/zero", { password });
    if (resp.code === 0) message.success(resp.msg);
    else message.error(resp.msg);
    setReCollectOpen(false);
    setPassword("");
  };

  const columns: ColumnsType<FilmSource> = [
    {
      title: "资源名称",
      dataIndex: "name",
      key: "name",
      render: (name: string, record) => (
        <Space>
          <span>{name}</span>
          {activeCollectIds.includes(record.id) && (
            <LoadingOutlined style={{ color: "var(--ant-color-primary)" }} />
          )}
        </Space>
      ),
    },
    {
      title: "数据类型",
      dataIndex: "resultModel",
      align: "center",
      render: (v: number) => <Tag>{v === 0 ? "JSON" : "XML"}</Tag>,
    },
    {
      title: "资源类型",
      dataIndex: "collectTypeText",
      align: "center",
      render: (v: string) => <Tag color="purple">{v}</Tag>,
    },
    {
      title: "资源站",
      dataIndex: "uri",
      ellipsis: true,
      render: (uri: string) => (
        <a href={uri} target="_blank" rel="noopener noreferrer">
          {uri}
        </a>
      ),
    },
    {
      title: "同步图片",
      dataIndex: "syncPictures",
      align: "center",
      render: (v: boolean, record) => (
        <Switch
          checked={v}
          disabled={record.grade === 1}
          onChange={(checked) => {
            record.syncPictures = checked;
            setSiteList([...siteList]);
            changeSourceState(record);
          }}
          checkedChildren="开启"
          unCheckedChildren="关闭"
        />
      ),
    },
    {
      title: "是否启用",
      dataIndex: "state",
      align: "center",
      render: (v: boolean, record) => (
        <Switch
          checked={v}
          onChange={(checked) => {
            record.state = checked;
            setSiteList([...siteList]);
            changeSourceState(record);
          }}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    {
      title: "站点权重",
      dataIndex: "grade",
      align: "center",
      render: (v: number) => (
        <Tag color={v === 0 ? "green" : "default"}>
          {v === 0 ? "采集主站" : "附属站点"}
        </Tag>
      ),
    },
    {
      title: "采集间隔",
      dataIndex: "interval",
      align: "center",
      render: (v: number) => (
        <Tag color="cyan">{v > 0 ? `${v} ms` : "无限制"}</Tag>
      ),
    },
    {
      title: "采集方式",
      width: 120,
      render: (_, record) => (
        <Select
          size="small"
          value={record.cd}
          onChange={(v) => {
            record.cd = v;
            setSiteList([...siteList]);
          }}
          style={{ width: "100%" }}
          options={collectDuration.map((d) => ({
            value: d.time,
            label: d.label,
          }))}
        />
      ),
    },
    {
      title: "操作",
      key: "action",
      align: "center",
      width: 160,
      fixed: "right",
      render: (_, record) => {
        const isRunning = activeCollectIds.includes(record.id);
        const isSlave = record.grade === 1;

        // 从站采集按钮（开始/截断）的渲染逻辑
        const renderStartBtn = () => {
          // 从站：主站无数据时完全不渲染采集按钮
          if (isSlave && !hasMasterData) return null;
          // 从站：主站采集中时渲染禁用按钮并提示原因
          if (isSlave && masterIsCollecting) {
            return (
              <Tooltip title="主站正在采集中，请等待主站采集完成后再采集从站">
                <Button
                  type="primary"
                  icon={<PoweroffOutlined />}
                  shape="circle"
                  size="small"
                  disabled
                />
              </Tooltip>
            );
          }
          // 正常情况：正在运行时显示截断重采，否则显示开始采集
          return isRunning ? (
            <Tooltip title="截断并重新开始">
              <Popconfirm
                title="该站点正在采集中"
                description="是否截断当前任务并重新开始？"
                okText="截断重采"
                cancelText="取消"
                onConfirm={() => startTask(record)}
              >
                <Button
                  type="primary"
                  icon={<PoweroffOutlined />}
                  shape="circle"
                  size="small"
                  style={{ background: "var(--ant-color-warning)", borderColor: "var(--ant-color-warning)" }}
                />
              </Popconfirm>
            </Tooltip>
          ) : (
            <Tooltip title="开始采集">
              <Button
                type="primary"
                icon={<PoweroffOutlined />}
                shape="circle"
                size="small"
                style={{ background: "var(--ant-color-success)", borderColor: "var(--ant-color-success)" }}
                onClick={() => startTask(record)}
              />
            </Tooltip>
          );
        };

        return (
          <Space>
            {renderStartBtn()}
            {isRunning && (
              <Tooltip title="停止采集">
                <Button
                  type="primary"
                  danger
                  icon={<PauseOutlined />}
                  shape="circle"
                  size="small"
                  onClick={() => stopTask(record.id)}
                />
              </Tooltip>
            )}
            <Button
              type="primary"
              icon={<EditOutlined />}
              shape="circle"
              size="small"
              onClick={() => openEditDialog(record.id)}
            />
            <Popconfirm
              title="确认删除此采集站？"
              onConfirm={() => delSource(record.id)}
            >
              <Button
                danger
                icon={<DeleteOutlined />}
                shape="circle"
                size="small"
              />
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  const commonFormItems = (
    <>
      <Form.Item label="资源名称" name="name" rules={[{ required: true }]}>
        <Input placeholder="自定义资源名称(禁用汉字)" />
      </Form.Item>
      <Form.Item label="接口地址" name="uri" rules={[{ required: true }]}>
        <Input placeholder="资源采集链接" />
      </Form.Item>
      <Form.Item label="间隔时长" name="interval" tooltip="单次请求的时间间隔, 单位/ms">
        <InputNumber min={0} step={100} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item label="接口类型" name="resultModel">
        <Radio.Group>
          <Radio value={0}>JSON</Radio>
          <Radio value={1} disabled>
            XML
          </Radio>
        </Radio.Group>
      </Form.Item>
      <Form.Item label="资源类型" name="collectType">
        <Radio.Group>
          <Radio value={0}>视频</Radio>
          <Radio value={1} disabled>
            文章
          </Radio>
          <Radio value={2} disabled>
            演员
          </Radio>
        </Radio.Group>
      </Form.Item>
      <Form.Item label="站点权重" name="grade">
        <Radio.Group
          onChange={(e) => {
            if (e.target.value === 1) form.setFieldValue("syncPictures", false);
          }}
        >
          <Radio value={0}>主站点</Radio>
          <Radio value={1}>附属站点</Radio>
        </Radio.Group>
      </Form.Item>
      <Form.Item label="图片同步" name="syncPictures" valuePropName="checked">
        <Switch checkedChildren="开启" unCheckedChildren="关闭" />
      </Form.Item>
      <Form.Item label="是否启用" name="state" valuePropName="checked">
        <Switch checkedChildren="启用" unCheckedChildren="禁用" />
      </Form.Item>
    </>
  );

  return (
    <div className={styles.container}>
      <Table
        columns={columns}
        dataSource={siteList}
        rowKey="id"
        loading={loading}
        bordered
        size="middle"
        pagination={false}
        scroll={{ x: "max-content" }}
      />

      <div className={styles.toolbar}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openAddDialog}>
          添加采集站
        </Button>
        <Tooltip title={!hasMasterData ? "主站暂无数据，请先采集主站后再进行一键采集" : ""}>
          <Button
            type="primary"
            icon={<SendOutlined />}
            style={{ background: "var(--ant-color-success)", borderColor: "var(--ant-color-success)" }}
            onClick={openBatchCollect}
            disabled={!hasMasterData}
          >
            一键采集
          </Button>
        </Tooltip>
        <Button
          icon={<ReloadOutlined />}
          style={{ color: "var(--ant-color-warning)", borderColor: "var(--ant-color-warning)" }}
          onClick={() => setReCollectOpen(true)}
        >
          清空重建
        </Button>
        <Button
          danger
          icon={<DeleteOutlined />}
          onClick={() => setClearOpen(true)}
        >
          清空数据
        </Button>
      </div>

      <Modal
        title="添加采集站点"
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        onOk={() => form.submit()}
        footer={[
          <Button key="test" type="dashed" onClick={testApi}>
            测试接口
          </Button>,
          <Button key="cancel" onClick={() => setAddOpen(false)}>
            取消
          </Button>,
          <Button key="ok" type="primary" onClick={() => form.submit()}>
            添加
          </Button>,
        ]}
      >
        <Form form={form} labelCol={{ span: 6 }} onFinish={onAddFinish}>
          {commonFormItems}
        </Form>
      </Modal>

      <Modal
        title="修改分类信息"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => form.submit()}
        footer={[
          <Button key="test" type="dashed" onClick={testApi}>
            测试接口
          </Button>,
          <Button key="cancel" onClick={() => setEditOpen(false)}>
            取消
          </Button>,
          <Button key="ok" type="primary" onClick={() => form.submit()}>
            更新
          </Button>,
        ]}
      >
        <Form form={form} labelCol={{ span: 6 }} onFinish={onEditFinish}>
          {commonFormItems}
        </Form>
      </Modal>

      <Modal
        title="多资源站一键采集"
        open={batchOpen}
        onCancel={() => setBatchOpen(false)}
        onOk={startBatchCollect}
        okText="确认执行"
      >
        {/* 正在采集中的站点提示 */}
        {(() => {
          const activeNames = enrichedBatchOptions
            .filter((o) => batchIds.includes(o.id) && activeCollectIds.includes(o.id))
            .map((o) => o.name);
          return activeNames.length > 0 ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="以下站点正在采集中，执行后将截断并重新开始"
              description={activeNames.join("、")}
            />
          ) : null;
        })()}
        {/* 执行站点列表（仅显示附属站） */}
        <Form layout="vertical">
          <Form.Item label="执行站点">
            <Checkbox.Group
              value={batchIds}
              onChange={(v) => setBatchIds(v as string[])}
            >
              <Space direction="vertical">
                {enrichedBatchOptions.map((o) => (
                  <Checkbox key={o.id} value={o.id}>
                    <Space size={4}>
                      <Tag
                        color={o.grade === 0 ? "green" : "default"}
                        style={{ marginRight: 0 }}
                      >
                        {o.grade === 0 ? "主站" : "从站"}
                      </Tag>
                      {o.name}
                      {activeCollectIds.includes(o.id) && (
                        <Tag
                          color="processing"
                          icon={<LoadingOutlined />}
                          style={{ marginLeft: 4 }}
                        >
                          采集中
                        </Tag>
                      )}
                    </Space>
                  </Checkbox>
                ))}
              </Space>
            </Checkbox.Group>
          </Form.Item>
          <Form.Item label="采集时长">
            <Select
              value={batchTime}
              onChange={setBatchTime}
              options={collectDuration.map((d) => ({
                label: d.label,
                value: d.time,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="清空影视数据"
        open={clearOpen}
        onCancel={() => setClearOpen(false)}
        onOk={clearFilms}
        okText="确认执行"
        okButtonProps={{ danger: true }}
      >
        <p style={{ color: "var(--ant-color-error)", marginBottom: 16 }}>
          此操作不可逆，将清空数据库中所有影片信息！
        </p>
        <Input.Password
          placeholder="请输入管理密码"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </Modal>

      <Modal
        title="清空数据并重建主站"
        open={reCollectOpen}
        onCancel={() => setReCollectOpen(false)}
        onOk={reCollect}
        okText="确认执行"
        okButtonProps={{ danger: true }}
      >
        <p style={{ color: "var(--ant-color-warning)", marginBottom: 16 }}>
          此操作将<strong>清空主站与从站的全部影视数据</strong>，随后仅对<strong>已启用的主站</strong>执行全量采集重建，从站数据由定时任务自动补充，操作不可逆。
        </p>
        <Input.Password
          placeholder="请输入管理密码"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </Modal>
    </div>
  );
}
