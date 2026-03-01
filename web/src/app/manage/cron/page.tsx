"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Table,
  Tag,
  Switch,
  Button,
  Space,
  Modal,
  Input,
  Form,
  Popconfirm,
  Select,
  InputNumber,
  Radio,
  Tooltip,
} from "antd";
import {
  ClockCircleOutlined,
  DeleteOutlined,
  EditOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ApiGet, ApiPost } from "@/lib/api";
import styles from "./page.module.less";
import { useAppMessage } from "@/lib/useAppMessage";

interface CronTask {
  id: string;
  cid: string;
  spec: string;
  remark: string;
  model: number;
  ids: string[];
  time: number;
  state: boolean;
  preV?: string;
  next?: string;
}

export default function CronManagePage() {
  const [taskList, setTaskList] = useState<CronTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [options, setOptions] = useState<any[]>([]);
  const { message } = useAppMessage();

  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [form] = Form.useForm();
  const [currentModel, setCurrentModel] = useState(1);

  const getTaskList = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await ApiGet("/manage/cron/list");
      if (resp.code === 0) {
        setTaskList(resp.data || []);
      } else {
        setTaskList([]);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const getOptions = async () => {
    const resp = await ApiGet("/manage/collect/options");
    if (resp.code === 0) {
      setOptions(resp.data || []);
    }
  };

  useEffect(() => {
    getTaskList();
  }, [getTaskList]);

  const changeTaskState = async (id: string, state: boolean) => {
    const resp = await ApiPost("/manage/cron/change", { id, state });
    if (resp.code === 0) {
      message.success(resp.msg);
      getTaskList();
    } else {
      message.error(resp.msg);
    }
  };

  const delTask = async (id: string) => {
    const resp = await ApiGet("/manage/cron/del", { id });
    if (resp.code === 0) {
      message.success(resp.msg);
      getTaskList();
    } else {
      message.error(resp.msg);
    }
  };

  const openAddDialog = () => {
    form.resetFields();
    form.setFieldsValue({ model: 1, state: false, time: 0, ids: [] });
    setCurrentModel(1);
    getOptions();
    setAddOpen(true);
  };

  const openEditDialog = async (id: string) => {
    form.resetFields();
    getOptions();
    const resp = await ApiGet("/manage/cron/find", { id });
    if (resp.code === 0) {
      form.setFieldsValue(resp.data);
      setCurrentModel(resp.data.model || 0);
      setEditOpen(true);
    } else {
      message.error(resp.msg);
    }
  };

  const onAddFinish = async (values: any) => {
    const resp = await ApiPost("/manage/cron/add", values);
    if (resp.code === 0) {
      message.success(resp.msg);
      setAddOpen(false);
      getTaskList();
    } else {
      message.error(resp.msg);
    }
  };

  const onEditFinish = async (values: any) => {
    const resp = await ApiPost("/manage/cron/update", {
      id: values.id,
      ids: values.ids,
      time: values.time,
      state: values.state,
      remark: values.remark,
    });
    if (resp.code === 0) {
      message.success(resp.msg);
      setEditOpen(false);
      getTaskList();
    } else {
      message.error(resp.msg);
    }
  };

  const columns: ColumnsType<CronTask> = [
    {
      title: "任务ID",
      dataIndex: "id",
      width: 200,
      render: (v) => <Tag color="purple">{v}</Tag>,
    },
    {
      title: "任务描述",
      dataIndex: "remark",
      ellipsis: true,
    },
    {
      title: "任务类型",
      dataIndex: "model",
      align: "center",
      render: (v) => (
        <Tag color="cyan">
          {v === 0 ? "自动更新" : v === 1 ? "自定义更新" : "采集重试"}
        </Tag>
      ),
    },
    {
      title: "是否启用",
      dataIndex: "state",
      align: "center",
      render: (v, record) => (
        <Switch
          checked={v}
          onChange={(checked) => changeTaskState(record.id, checked)}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    {
      title: "上次执行时间",
      dataIndex: "preV",
      align: "center",
      render: (v) => <Tag color="success">{v || "-"}</Tag>,
    },
    {
      title: "下次执行时间",
      dataIndex: "next",
      align: "center",
      render: (v) => <Tag color="warning">{v || "-"}</Tag>,
    },
    {
      title: "操作",
      key: "action",
      align: "center",
      width: 120,
      fixed: "right",
      render: (_, record) => (
        <Space>
          <Button
            type="primary"
            icon={<EditOutlined />}
            shape="circle"
            size="small"
            onClick={() => openEditDialog(record.id)}
          />
          <Popconfirm
            title="确认删除该定时任务？"
            onConfirm={() => delTask(record.id)}
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

  const commonFormItems = (isEdit: boolean = false) => (
    <>
      <Form.Item name="id" hidden>
        <Input />
      </Form.Item>
      {isEdit && (
        <Form.Item label="任务标识" name="id">
          <Input disabled />
        </Form.Item>
      )}
      {!isEdit && (
        <Form.Item
          label="任务周期"
          name="spec"
          rules={[{ required: true, message: "请输入Cron表达式" }]}
        >
          <Input placeholder="例如: 0 */20 * * * ? (每20分钟执行一次)" />
        </Form.Item>
      )}
      {isEdit && (
        <Form.Item label="任务周期" name="spec">
          <Input disabled />
        </Form.Item>
      )}
      <Form.Item
        label="任务描述"
        name="remark"
        rules={[{ required: true, message: "请输入任务描述" }]}
      >
        <Input placeholder="定时任务描述信息" />
      </Form.Item>
      <Form.Item label="任务类型" name="model">
        <Radio.Group
          onChange={(e) => setCurrentModel(e.target.value)}
          disabled={isEdit}
        >
          <Tooltip title="执行所有已启用站点的采集任务">
            <Radio value={0}>自动更新</Radio>
          </Tooltip>
          <Tooltip title="只执行指定站点的采集任务">
            <Radio value={1}>自定义更新</Radio>
          </Tooltip>
          <Tooltip title="失败采集重试处理">
            <Radio value={2}>采集重试</Radio>
          </Tooltip>
        </Radio.Group>
      </Form.Item>
      {currentModel === 1 && (
        <Form.Item label="资源绑定" name="ids">
          <Select
            mode="multiple"
            placeholder="请选择绑定的采集站"
            style={{ width: "100%" }}
            options={options.map((o) => ({ label: o.name, value: o.id }))}
          />
        </Form.Item>
      )}
      {currentModel !== 2 && (
        <Form.Item label="采集时长" name="time">
          <InputNumber style={{ width: "100%" }} placeholder="负数则默认全量" />
        </Form.Item>
      )}
      <Form.Item label="任务状态" name="state" valuePropName="checked">
        <Switch checkedChildren="开启" unCheckedChildren="禁用" />
      </Form.Item>
    </>
  );

  return (
    <div>
      <Table
        columns={columns}
        dataSource={taskList}
        rowKey="id"
        loading={loading}
        bordered
        size="middle"
        pagination={false}
        scroll={{ x: "max-content" }}
      />
      <div className={styles.toolbar}>
        <Button
          type="primary"
          icon={<ClockCircleOutlined />}
          style={{ background: "#9b49e7", borderColor: "#9b49e7" }}
          onClick={openAddDialog}
        >
          创建定时任务
        </Button>
      </div>

      <Modal
        title="创建定时任务"
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        onOk={() => form.validateFields().then(onAddFinish)}
        okButtonProps={{
          style: { background: "#9b49e7", borderColor: "#9b49e7" },
        }}
      >
        <Form form={form} layout="vertical">
          {commonFormItems(false)}
        </Form>
      </Modal>

      <Modal
        title="编辑定时任务"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => form.validateFields().then(onEditFinish)}
        okButtonProps={{
          style: { background: "#9b49e7", borderColor: "#9b49e7" },
        }}
      >
        <Form form={form} layout="vertical">
          {commonFormItems(true)}
        </Form>
      </Modal>
    </div>
  );
}
